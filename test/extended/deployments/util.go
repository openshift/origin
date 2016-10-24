package deployments

import (
	"fmt"
	"sort"
	"strings"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	exutil "github.com/openshift/origin/test/extended/util"
)

func deploymentStatuses(rcs []kapi.ReplicationController) []string {
	statuses := []string{}
	for _, rc := range rcs {
		statuses = append(statuses, string(deployutil.DeploymentStatusFor(&rc)))
	}
	return statuses
}

func deploymentPods(pods []kapi.Pod) (map[string][]*kapi.Pod, error) {
	deployers := make(map[string][]*kapi.Pod)
	for i := range pods {
		name, ok := pods[i].Labels[deployapi.DeployerPodForDeploymentLabel]
		if !ok {
			continue
		}
		deployers[name] = append(deployers[name], &pods[i])
	}
	return deployers, nil
}

var completedStatuses = sets.NewString(string(deployapi.DeploymentStatusComplete), string(deployapi.DeploymentStatusFailed))

func checkDeployerPodInvariants(deploymentName string, pods []*kapi.Pod) (isRunning, isCompleted bool, err error) {
	running := false
	completed := false
	succeeded := false
	hasDeployer := false

	// find deployment state
	for _, pod := range pods {
		switch {
		case strings.HasSuffix(pod.Name, "-deploy"):
			if hasDeployer {
				return false, false, fmt.Errorf("multiple deployer pods for %q", deploymentName)
			}
			hasDeployer = true

			switch pod.Status.Phase {
			case kapi.PodSucceeded:
				succeeded = true
				completed = true
			case kapi.PodFailed:
				completed = true
			default:
				running = true
			}
		case strings.HasSuffix(pod.Name, "-pre"), strings.HasSuffix(pod.Name, "-mid"), strings.HasSuffix(pod.Name, "-post"):
		default:
			return false, false, fmt.Errorf("deployer pod %q not recognized as being a valid deployment pod", pod.Name)
		}
	}

	// check hook pods
	for _, pod := range pods {
		switch {
		case strings.HasSuffix(pod.Name, "-pre"), strings.HasSuffix(pod.Name, "-mid"), strings.HasSuffix(pod.Name, "-post"):
			switch pod.Status.Phase {
			case kapi.PodSucceeded:
			case kapi.PodFailed:
				if succeeded {
					return false, false, fmt.Errorf("deployer hook pod %q failed but the deployment %q pod succeeded", pod.Name, deploymentName)
				}
			default:
				if completed {
					// TODO: we need to tighten guarantees around hook pods: https://github.com/openshift/origin/issues/8500
					//for i := range pods {
					//	e2e.Logf("deployment %q pod[%d]: %#v", deploymentName, i, pods[i])
					//}
					//return false, false, fmt.Errorf("deployer hook pod %q is still running but the deployment %q is complete", pod.Name, deploymentName)
					//e2e.Logf("deployer hook pod %q is still running but the deployment %q is complete", pod.Name, deploymentName)
				}
			}
		}
	}
	return running, completed, nil
}

func checkDeploymentInvariants(dc *deployapi.DeploymentConfig, rcs []kapi.ReplicationController, pods []kapi.Pod) error {
	deployers, err := deploymentPods(pods)
	if err != nil {
		return err
	}
	if len(deployers) > len(rcs) {
		existing := sets.NewString()
		for k := range deployers {
			existing.Insert(k)
		}
		for _, rc := range rcs {
			if existing.Has(rc.Name) {
				existing.Delete(rc.Name)
			} else {
				e2e.Logf("ANOMALY: No deployer pod found for deployment %q", rc.Name)
			}
		}
		for k := range existing {
			// TODO: we are missing RCs? https://github.com/openshift/origin/pull/8483#issuecomment-209150611
			e2e.Logf("ANOMALY: Deployer pod found for %q but no RC exists", k)
			//return fmt.Errorf("more deployer pods found than deployments: %#v %#v", deployers, rcs)
		}
	}
	running := sets.NewString()
	completed := 0
	for k, v := range deployers {
		isRunning, isCompleted, err := checkDeployerPodInvariants(k, v)
		if err != nil {
			return err
		}
		if isCompleted {
			completed++
		}
		if isRunning {
			running.Insert(k)
		}
	}
	if running.Len() > 1 {
		return fmt.Errorf("found multiple running deployments: %v", running.List())
	}
	sawStatus := sets.NewString()
	statuses := []string{}
	for _, rc := range rcs {
		status := deployutil.DeploymentStatusFor(&rc)
		if sawStatus.Len() != 0 {
			switch status {
			case deployapi.DeploymentStatusComplete, deployapi.DeploymentStatusFailed:
				if sawStatus.Difference(completedStatuses).Len() != 0 {
					return fmt.Errorf("rc %s was %s, but earlier RCs were not completed: %v", rc.Name, status, statuses)
				}
			case deployapi.DeploymentStatusRunning, deployapi.DeploymentStatusPending:
				if sawStatus.Has(string(status)) {
					return fmt.Errorf("rc %s was %s, but so was an earlier RC: %v", rc.Name, status, statuses)
				}
				if sawStatus.Difference(completedStatuses).Len() != 0 {
					return fmt.Errorf("rc %s was %s, but earlier RCs were not completed: %v", rc.Name, status, statuses)
				}
			case deployapi.DeploymentStatusNew:
			default:
				return fmt.Errorf("rc %s has unexpected status %s: %v", rc.Name, status, statuses)
			}
		}
		sawStatus.Insert(string(status))
		statuses = append(statuses, string(status))
	}
	return nil
}

func deploymentReachedCompletion(dc *deployapi.DeploymentConfig, rcs []kapi.ReplicationController, pods []kapi.Pod) (bool, error) {
	if len(rcs) == 0 {
		return false, nil
	}
	rc := rcs[len(rcs)-1]
	version := deployutil.DeploymentVersionFor(&rc)
	if version != dc.Status.LatestVersion {
		return false, nil
	}

	if !deployutil.IsCompleteDeployment(&rc) {
		return false, nil
	}
	cond := deployutil.GetDeploymentCondition(dc.Status, deployapi.DeploymentProgressing)
	if cond == nil || cond.Reason != deployutil.NewRcAvailableReason {
		return false, nil
	}
	expectedReplicas := dc.Spec.Replicas
	if dc.Spec.Test {
		expectedReplicas = 0
	}
	if rc.Spec.Replicas != int32(expectedReplicas) {
		return false, fmt.Errorf("deployment is complete but doesn't have expected spec replicas: %d %d", rc.Spec.Replicas, expectedReplicas)
	}
	if rc.Status.Replicas != int32(expectedReplicas) {
		e2e.Logf("POSSIBLE_ANOMALY: deployment is complete but doesn't have expected status replicas: %d %d", rc.Status.Replicas, expectedReplicas)
		return false, nil
	}
	e2e.Logf("Latest rollout of dc/%s (rc/%s) is complete.", dc.Name, rc.Name)
	return true, nil
}

func deploymentFailed(dc *deployapi.DeploymentConfig, rcs []kapi.ReplicationController, _ []kapi.Pod) (bool, error) {
	if len(rcs) == 0 {
		return false, nil
	}
	rc := rcs[len(rcs)-1]
	version := deployutil.DeploymentVersionFor(&rc)
	if version != dc.Status.LatestVersion {
		return false, nil
	}
	if !deployutil.IsFailedDeployment(&rc) {
		return false, nil
	}
	cond := deployutil.GetDeploymentCondition(dc.Status, deployapi.DeploymentProgressing)
	return cond != nil && cond.Reason == deployutil.TimedOutReason, nil
}

func deploymentRunning(dc *deployapi.DeploymentConfig, rcs []kapi.ReplicationController, pods []kapi.Pod) (bool, error) {
	if len(rcs) == 0 {
		return false, nil
	}
	rc := rcs[len(rcs)-1]
	version := deployutil.DeploymentVersionFor(&rc)
	if version != dc.Status.LatestVersion {
		//e2e.Logf("deployment %s is not the latest version on DC: %d", rc.Name, version)
		return false, nil
	}

	status := rc.Annotations[deployapi.DeploymentStatusAnnotation]
	switch deployapi.DeploymentStatus(status) {
	case deployapi.DeploymentStatusFailed:
		if deployutil.IsDeploymentCancelled(&rc) {
			return true, nil
		}
		reason := deployutil.DeploymentStatusReasonFor(&rc)
		if reason == "deployer pod no longer exists" {
			return true, nil
		}
		return false, fmt.Errorf("deployment failed: %v", deployutil.DeploymentStatusReasonFor(&rc))
	case deployapi.DeploymentStatusRunning, deployapi.DeploymentStatusComplete:
		return true, nil
	default:
		return false, nil
	}
}

func deploymentPreHookRetried(dc *deployapi.DeploymentConfig, rcs []kapi.ReplicationController, pods []kapi.Pod) (bool, error) {
	var preHook *kapi.Pod
	for i := range pods {
		pod := pods[i]
		if !strings.HasSuffix(pod.Name, "-pre") {
			continue
		}
		preHook = &pod
		break
	}

	if preHook == nil || len(preHook.Status.ContainerStatuses) == 0 {
		return false, nil
	}

	return preHook.Status.ContainerStatuses[0].RestartCount > 0, nil
}

func deploymentInfo(oc *exutil.CLI, name string) (*deployapi.DeploymentConfig, []kapi.ReplicationController, []kapi.Pod, error) {
	dc, err := oc.REST().DeploymentConfigs(oc.Namespace()).Get(name)
	if err != nil {
		return nil, nil, nil, err
	}

	// get pods before RCs, so we see more RCs than pods.
	pods, err := oc.KubeREST().Pods(oc.Namespace()).List(kapi.ListOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	rcs, err := oc.KubeREST().ReplicationControllers(oc.Namespace()).List(kapi.ListOptions{
		LabelSelector: deployutil.ConfigSelector(name),
	})
	if err != nil {
		return nil, nil, nil, err
	}
	sort.Sort(deployutil.ByLatestVersionAsc(rcs.Items))

	return dc, rcs.Items, pods.Items, nil
}

type deploymentConditionFunc func(dc *deployapi.DeploymentConfig, rcs []kapi.ReplicationController, pods []kapi.Pod) (bool, error)

func waitForLatestCondition(oc *exutil.CLI, name string, timeout time.Duration, fn deploymentConditionFunc) error {
	return wait.PollImmediate(200*time.Millisecond, timeout, func() (bool, error) {
		dc, rcs, pods, err := deploymentInfo(oc, name)
		if err != nil {
			return false, err
		}
		if err := checkDeploymentInvariants(dc, rcs, pods); err != nil {
			return false, err
		}
		return fn(dc, rcs, pods)
	})
}

func waitForSyncedConfig(oc *exutil.CLI, name string, timeout time.Duration) error {
	dc, rcs, pods, err := deploymentInfo(oc, name)
	if err != nil {
		return err
	}
	if err := checkDeploymentInvariants(dc, rcs, pods); err != nil {
		return err
	}
	generation := dc.Generation
	return wait.PollImmediate(200*time.Millisecond, timeout, func() (bool, error) {
		config, err := oc.REST().DeploymentConfigs(oc.Namespace()).Get(name)
		if err != nil {
			return false, err
		}
		return deployutil.HasSynced(config, generation), nil
	})
}

// createFixture will create the provided fixture and return the resource and the
// name separately.
// TODO: Probably move to a more general location like test/extended/util/cli.go
func createFixture(oc *exutil.CLI, fixture string) (string, string, error) {
	resource, err := oc.Run("create").Args("-f", fixture, "-o", "name").Output()
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(resource, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected type/name syntax, got: %s", resource)
	}
	return resource, parts[1], nil
}

func failureTrap(oc *exutil.CLI, name string, failed bool) {
	if !failed {
		return
	}
	out, err := oc.Run("get").Args("dc/"+name, "-o", "yaml").Output()
	if err != nil {
		e2e.Logf("Error getting Deployment Config %s: %v", name, err)
		return
	}
	e2e.Logf("\n%s\n", out)
	_, rcs, pods, err := deploymentInfo(oc, name)
	if err != nil {
		e2e.Logf("Error getting deployment %s info: %v", name, err)
		return
	}
	for _, r := range rcs {
		out, err := oc.Run("get").Args("rc/"+r.Name, "-o", "yaml").Output()
		if err != nil {
			e2e.Logf("Error getting replication controller %s info: %v", r.Name, err)
			return
		}
		e2e.Logf("\n%s\n", out)
	}
	p, _ := deploymentPods(pods)
	for _, v := range p {
		for _, pod := range v {
			out, err := oc.Run("get").Args("pod/"+pod.Name, "-o", "yaml").Output()
			if err != nil {
				e2e.Logf("Error getting pod %s: %v", pod.Name, err)
				return
			}
			e2e.Logf("\n%s\n", out)
			out, _ = oc.Run("logs").Args("pod/"+pod.Name, "--timestamps=true").Output()
			e2e.Logf("--- pod %s logs\n%s---\n", pod.Name, out)
		}
	}
}
