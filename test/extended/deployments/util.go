package deployments

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/ghodss/yaml"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	kcontroller "k8s.io/kubernetes/pkg/controller"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	deployapiv1 "github.com/openshift/origin/pkg/deploy/apis/apps/v1"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func deploymentPods(pods []kapiv1.Pod) (map[string][]*kapiv1.Pod, error) {
	deployers := make(map[string][]*kapiv1.Pod)
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

func checkDeployerPodInvariants(deploymentName string, pods []*kapiv1.Pod) (isRunning, isCompleted bool, err error) {
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
			case kapiv1.PodSucceeded:
				succeeded = true
				completed = true
			case kapiv1.PodFailed:
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
			case kapiv1.PodSucceeded:
			case kapiv1.PodFailed:
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

func checkDeploymentInvariants(dc *deployapi.DeploymentConfig, rcs []*kapiv1.ReplicationController, pods []kapiv1.Pod) error {
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
		status := deployutil.DeploymentStatusFor(rc)
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

func deploymentReachedCompletion(dc *deployapi.DeploymentConfig, rcs []*kapiv1.ReplicationController, pods []kapiv1.Pod) (bool, error) {
	if len(rcs) == 0 {
		return false, nil
	}
	rcv1 := rcs[len(rcs)-1]
	rc := &kapi.ReplicationController{}
	kapiv1.Convert_v1_ReplicationController_To_api_ReplicationController(rcv1, rc, nil)
	version := deployutil.DeploymentVersionFor(rc)
	if version != dc.Status.LatestVersion {
		return false, nil
	}

	if !deployutil.IsCompleteDeployment(rc) {
		return false, nil
	}
	cond := deployutil.GetDeploymentCondition(dc.Status, deployapi.DeploymentProgressing)
	if cond == nil || cond.Reason != deployapi.NewRcAvailableReason {
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

func deploymentFailed(dc *deployapi.DeploymentConfig, rcs []*kapiv1.ReplicationController, _ []kapiv1.Pod) (bool, error) {
	if len(rcs) == 0 {
		return false, nil
	}
	rcv1 := rcs[len(rcs)-1]
	rc := &kapi.ReplicationController{}
	kapiv1.Convert_v1_ReplicationController_To_api_ReplicationController(rcv1, rc, nil)
	version := deployutil.DeploymentVersionFor(rc)
	if version != dc.Status.LatestVersion {
		return false, nil
	}
	if !deployutil.IsFailedDeployment(rc) {
		return false, nil
	}
	cond := deployutil.GetDeploymentCondition(dc.Status, deployapi.DeploymentProgressing)
	return cond != nil && cond.Reason == deployapi.TimedOutReason, nil
}

func deploymentRunning(dc *deployapi.DeploymentConfig, rcs []*kapiv1.ReplicationController, pods []kapiv1.Pod) (bool, error) {
	if len(rcs) == 0 {
		return false, nil
	}
	rcv1 := rcs[len(rcs)-1]
	rc := &kapi.ReplicationController{}
	kapiv1.Convert_v1_ReplicationController_To_api_ReplicationController(rcv1, rc, nil)
	version := deployutil.DeploymentVersionFor(rc)
	if version != dc.Status.LatestVersion {
		//e2e.Logf("deployment %s is not the latest version on DC: %d", rc.Name, version)
		return false, nil
	}

	status := rc.Annotations[deployapi.DeploymentStatusAnnotation]
	switch deployapi.DeploymentStatus(status) {
	case deployapi.DeploymentStatusFailed:
		if deployutil.IsDeploymentCancelled(rc) {
			return true, nil
		}
		reason := deployutil.DeploymentStatusReasonFor(rc)
		if reason == "deployer pod no longer exists" {
			return true, nil
		}
		return false, fmt.Errorf("deployment failed: %v", deployutil.DeploymentStatusReasonFor(rc))
	case deployapi.DeploymentStatusRunning, deployapi.DeploymentStatusComplete:
		return true, nil
	default:
		return false, nil
	}
}

func deploymentPreHookRetried(dc *deployapi.DeploymentConfig, rcs []*kapiv1.ReplicationController, pods []kapiv1.Pod) (bool, error) {
	var preHook *kapiv1.Pod
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

func deploymentImageTriggersResolved(expectTriggers int) func(dc *deployapi.DeploymentConfig, rcs []*kapiv1.ReplicationController, pods []kapiv1.Pod) (bool, error) {
	return func(dc *deployapi.DeploymentConfig, rcs []*kapiv1.ReplicationController, pods []kapiv1.Pod) (bool, error) {
		expect := 0
		for _, t := range dc.Spec.Triggers {
			if t.Type != deployapi.DeploymentTriggerOnImageChange {
				continue
			}
			if expect >= expectTriggers {
				return false, fmt.Errorf("dc %s had too many image change triggers: %#v", dc.Name, dc.Spec.Triggers)
			}
			if t.ImageChangeParams == nil {
				return false, nil
			}
			if len(t.ImageChangeParams.LastTriggeredImage) == 0 {
				return false, nil
			}
			expect++
		}
		return expect == expectTriggers, nil
	}
}

func deploymentInfo(oc *exutil.CLI, name string) (*deployapi.DeploymentConfig, []*kapiv1.ReplicationController, []kapiv1.Pod, error) {
	dc, err := oc.Client().DeploymentConfigs(oc.Namespace()).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	// get pods before RCs, so we see more RCs than pods.
	pods, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).List(metav1.ListOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	rcs, err := oc.KubeClient().CoreV1().ReplicationControllers(oc.Namespace()).List(metav1.ListOptions{
		LabelSelector: deployutil.ConfigSelector(name).String(),
	})
	if err != nil {
		return nil, nil, nil, err
	}

	deployments := make([]*kapiv1.ReplicationController, 0, len(rcs.Items))
	for i := range rcs.Items {
		deployments = append(deployments, &rcs.Items[i])
	}

	sort.Sort(deployutil.ByLatestVersionAscV1(deployments))

	return dc, deployments, pods.Items, nil
}

type deploymentConditionFunc func(dc *deployapi.DeploymentConfig, rcs []*kapiv1.ReplicationController, pods []kapiv1.Pod) (bool, error)

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
		config, err := oc.Client().DeploymentConfigs(oc.Namespace()).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return deployutil.HasSynced(config, generation), nil
	})
}

// waitForDeployerToComplete waits till the replication controller is created for a given
// rollout and then wait till the deployer pod finish. Then scrubs the deployer logs and
// return it.
func waitForDeployerToComplete(oc *exutil.CLI, name string, timeout time.Duration) (string, error) {
	watcher, err := oc.InternalKubeClient().Core().ReplicationControllers(oc.Namespace()).Watch(metav1.ListOptions{FieldSelector: fields.Everything().String()})
	if err != nil {
		return "", err
	}
	defer watcher.Stop()
	var rc *kapi.ReplicationController
	if _, err := watch.Until(timeout, watcher, func(e watch.Event) (bool, error) {
		if e.Type == watch.Error {
			return false, fmt.Errorf("error while waiting for replication controller: %v", e.Object)
		}
		if e.Type == watch.Added || e.Type == watch.Modified {
			if newRC, ok := e.Object.(*kapi.ReplicationController); ok && newRC.Name == name {
				rc = newRC
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return "", err
	}
	podName := deployutil.DeployerPodNameForDeployment(rc.Name)
	if err := deployutil.WaitForRunningDeployerPod(oc.InternalKubeClient().Core(), rc, timeout); err != nil {
		return "", err
	}
	output, err := oc.Run("logs").Args("-f", "pods/"+podName).Output()
	if err != nil {
		return "", err
	}
	return output, nil
}

func isControllerRefChange(controllee metav1.Object, old *metav1.OwnerReference) (bool, error) {
	if old != nil && old.Controller != nil && *old.Controller == false {
		return false, fmt.Errorf("old ownerReference is not a controllerRef")
	}
	return !reflect.DeepEqual(old, kcontroller.GetControllerOf(controllee)), nil
}

func controllerRefChangeCondition(old *metav1.OwnerReference) func(controllee metav1.Object) (bool, error) {
	return func(controllee metav1.Object) (bool, error) {
		return isControllerRefChange(controllee, old)
	}
}

func rCConditionFromMeta(condition func(metav1.Object) (bool, error)) func(rc *kapiv1.ReplicationController) (bool, error) {
	return func(rc *kapiv1.ReplicationController) (bool, error) {
		return condition(rc)
	}
}

func waitForRCModification(oc *exutil.CLI, namespace string, name string, timeout time.Duration, resourceVersion string, condition func(rc *kapiv1.ReplicationController) (bool, error)) (*kapiv1.ReplicationController, error) {
	watcher, err := oc.KubeClient().CoreV1().ReplicationControllers(namespace).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: name, ResourceVersion: resourceVersion}))
	if err != nil {
		return nil, err
	}

	event, err := watch.Until(timeout, watcher, func(event watch.Event) (bool, error) {
		if event.Type != watch.Modified {
			return false, fmt.Errorf("different kind of event appeared while waiting for modification: event: %#v", event)
		}
		return condition(event.Object.(*kapiv1.ReplicationController))
	})
	if err != nil {
		return nil, err
	}
	if event.Type != watch.Modified {
		return nil, fmt.Errorf("waiting for RC modification failed: event: %v", event)
	}
	return event.Object.(*kapiv1.ReplicationController), nil
}

func waitForDCModification(oc *exutil.CLI, namespace string, name string, timeout time.Duration, resourceVersion string, condition func(rc *deployapi.DeploymentConfig) (bool, error)) (*deployapi.DeploymentConfig, error) {
	watcher, err := oc.Client().DeploymentConfigs(namespace).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: name, ResourceVersion: resourceVersion}))
	if err != nil {
		return nil, err
	}

	event, err := watch.Until(timeout, watcher, func(event watch.Event) (bool, error) {
		if event.Type != watch.Modified {
			return false, fmt.Errorf("different kind of event appeared while waiting for modification: event: %#v", event)
		}
		return condition(event.Object.(*deployapi.DeploymentConfig))
	})
	if err != nil {
		return nil, err
	}
	if event.Type != watch.Modified {
		return nil, fmt.Errorf("waiting for DC modification failed: event: %v", event)
	}
	return event.Object.(*deployapi.DeploymentConfig), nil
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

func createDeploymentConfig(oc *exutil.CLI, fixture string) (*deployapi.DeploymentConfig, error) {
	_, name, err := createFixture(oc, fixture)
	if err != nil {
		return nil, err
	}
	var pollErr error
	var dc *deployapi.DeploymentConfig
	err = wait.PollImmediate(1*time.Second, 1*time.Minute, func() (bool, error) {
		dc, err = oc.Client().DeploymentConfigs(oc.Namespace()).Get(name, metav1.GetOptions{})
		if err != nil {
			pollErr = err
			return false, nil
		}
		return true, nil
	})
	if err == wait.ErrWaitTimeout {
		err = pollErr
	}
	return dc, err
}

func DeploymentConfigFailureTrap(oc *exutil.CLI, name string, failed bool) {
	failureTrap(oc, name, failed)
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

func failureTrapForDetachedRCs(oc *exutil.CLI, dcName string, failed bool) {
	if !failed {
		return
	}
	kclient := oc.KubeClient()
	requirement, err := labels.NewRequirement(deployapi.DeploymentConfigAnnotation, selection.NotEquals, []string{dcName})
	if err != nil {
		e2e.Logf("failed to create requirement for DC %q", dcName)
		return
	}
	dc, err := kclient.CoreV1().ReplicationControllers(oc.Namespace()).List(metav1.ListOptions{
		LabelSelector: labels.NewSelector().Add(*requirement).String(),
	})
	if err != nil {
		e2e.Logf("Error getting detached RCs; DC %q: %v", dcName, err)
		return
	}
	if len(dc.Items) == 0 {
		e2e.Logf("No detached RCs found.")
	} else {
		out, err := oc.Run("get").Args("rc", "-o", "yaml", "-l", fmt.Sprintf("%s!=%s", deployapi.DeploymentConfigAnnotation, dcName)).Output()
		if err != nil {
			e2e.Logf("Failed to list detached RCs!")
			return
		}
		e2e.Logf("There are detached RCs: \n%s", out)
	}
}

// Checks controllerRef from controllee to DC.
// Return true is the controllerRef is valid, false otherwise
func HasValidDCControllerRef(dc metav1.Object, controllee metav1.Object) bool {
	ref := kcontroller.GetControllerOf(controllee)
	return ref != nil &&
		ref.UID == dc.GetUID() &&
		ref.APIVersion == deployutil.DeploymentConfigControllerRefKind.GroupVersion().String() &&
		ref.Kind == deployutil.DeploymentConfigControllerRefKind.Kind &&
		ref.Name == dc.GetName()
}

func readDCFixture(path string) (*deployapi.DeploymentConfig, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	dcv1 := new(deployapiv1.DeploymentConfig)
	err = yaml.Unmarshal(data, dcv1)
	if err != nil {
		return nil, err
	}

	dc := new(deployapi.DeploymentConfig)
	err = deployapiv1.Convert_v1_DeploymentConfig_To_apps_DeploymentConfig(dcv1, dc, nil)
	return dc, err
}

func readDCFixtureOrDie(path string) *deployapi.DeploymentConfig {
	data, err := readDCFixture(path)
	if err != nil {
		panic(err)
	}
	return data
}
