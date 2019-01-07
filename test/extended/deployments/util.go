package deployments

import (
	"context"
	"fmt"
	"io/ioutil"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	appsv1 "github.com/openshift/api/apps/v1"
	appstypedclient "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	"github.com/openshift/origin/pkg/apps/apiserver/registry/deploylog"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	exutil "github.com/openshift/origin/test/extended/util"
)

type updateConfigFunc func(d *appsv1.DeploymentConfig)

// updateConfigWithRetries will try to update a deployment config and ignore any update conflicts.
func updateConfigWithRetries(dn appstypedclient.DeploymentConfigsGetter, namespace, name string, applyUpdate updateConfigFunc) (*appsv1.DeploymentConfig, error) {
	var config *appsv1.DeploymentConfig
	resultErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var err error
		config, err = dn.DeploymentConfigs(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		// Apply the update, then attempt to push it to the apiserver.
		applyUpdate(config)
		config, err = dn.DeploymentConfigs(namespace).Update(config)
		return err
	})
	return config, resultErr
}

func deploymentPods(pods []corev1.Pod) (map[string][]*corev1.Pod, error) {
	deployers := make(map[string][]*corev1.Pod)
	for i := range pods {
		name, ok := pods[i].Labels[appsv1.DeployerPodForDeploymentLabel]
		if !ok {
			continue
		}
		deployers[name] = append(deployers[name], &pods[i])
	}
	return deployers, nil
}

var completedStatuses = sets.NewString(string(appsv1.DeploymentStatusComplete), string(appsv1.DeploymentStatusFailed))

func checkDeployerPodInvariants(deploymentName string, pods []*corev1.Pod) (isRunning, isCompleted bool, err error) {
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
			case corev1.PodSucceeded:
				succeeded = true
				completed = true
			case corev1.PodFailed:
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
			case corev1.PodSucceeded:
			case corev1.PodFailed:
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

func checkDeploymentInvariants(dc *appsv1.DeploymentConfig, rcs []*corev1.ReplicationController, pods []corev1.Pod) error {
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
		status := appsutil.DeploymentStatusFor(rc)
		if sawStatus.Len() != 0 {
			switch status {
			case appsv1.DeploymentStatusComplete, appsv1.DeploymentStatusFailed:
				if sawStatus.Difference(completedStatuses).Len() != 0 {
					return fmt.Errorf("rc %s was %s, but earlier RCs were not completed: %v", rc.Name, status, statuses)
				}
			case appsv1.DeploymentStatusRunning, appsv1.DeploymentStatusPending:
				if sawStatus.Has(string(status)) {
					return fmt.Errorf("rc %s was %s, but so was an earlier RC: %v", rc.Name, status, statuses)
				}
				if sawStatus.Difference(completedStatuses).Len() != 0 {
					return fmt.Errorf("rc %s was %s, but earlier RCs were not completed: %v", rc.Name, status, statuses)
				}
			case appsv1.DeploymentStatusNew:
			default:
				return fmt.Errorf("rc %s has unexpected status %s: %v", rc.Name, status, statuses)
			}
		}
		sawStatus.Insert(string(status))
		statuses = append(statuses, string(status))
	}
	return nil
}

// GetDeploymentCondition returns the condition with the provided type.
// DEPRECATED: Will be removed when extended tests move to external
func GetDeploymentCondition(status appsv1.DeploymentConfigStatus, condType appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

func deploymentReachedCompletion(dc *appsv1.DeploymentConfig, rcs []*corev1.ReplicationController, pods []corev1.Pod) (bool, error) {
	if dc.Status.ObservedGeneration != dc.Generation {
		return false, nil
	}
	if len(rcs) == 0 {
		return false, nil
	}
	rc := rcs[len(rcs)-1]
	version := appsutil.DeploymentVersionFor(rc)
	if version != dc.Status.LatestVersion {
		return false, nil
	}

	if appsutil.IsFailedDeployment(rc) {
		return true, fmt.Errorf("deployment %s/%s failed", rc.Namespace, rc.Name)
	}

	if !appsutil.IsCompleteDeployment(rc) {
		return false, nil
	}
	cond := GetDeploymentCondition(dc.Status, appsv1.DeploymentProgressing)
	if cond == nil || cond.Reason != appsutil.NewRcAvailableReason {
		return false, nil
	}
	zeroReplicas := int32(0)
	expectedReplicas := &dc.Spec.Replicas
	if dc.Spec.Test {
		expectedReplicas = &zeroReplicas
	}
	if *rc.Spec.Replicas != *expectedReplicas {
		return false, fmt.Errorf("deployment is complete but doesn't have expected spec replicas: %d %d", *rc.Spec.Replicas, *expectedReplicas)
	}
	if expectedReplicas == nil {
		return false, fmt.Errorf("expectedReplicas should not be nil")
	}
	if rc.Status.Replicas != *expectedReplicas {
		e2e.Logf("POSSIBLE_ANOMALY: deployment is complete but doesn't have expected status replicas: %d %d", rc.Status.Replicas, *expectedReplicas)
		return false, nil
	}
	e2e.Logf("Latest rollout of dc/%s (rc/%s) is complete.", dc.Name, rc.Name)
	return true, nil
}

func deploymentFailed(dc *appsv1.DeploymentConfig, rcs []*corev1.ReplicationController, _ []corev1.Pod) (bool, error) {
	if len(rcs) == 0 {
		return false, nil
	}
	rc := rcs[len(rcs)-1]
	version := appsutil.DeploymentVersionFor(rc)
	if version != dc.Status.LatestVersion {
		return false, nil
	}
	if !appsutil.IsFailedDeployment(rc) {
		return false, nil
	}
	cond := appsutil.GetDeploymentCondition(dc.Status, appsv1.DeploymentProgressing)
	return cond != nil && cond.Reason == appsutil.TimedOutReason, nil
}

func deploymentRunning(dc *appsv1.DeploymentConfig, rcs []*corev1.ReplicationController, pods []corev1.Pod) (bool, error) {
	if len(rcs) == 0 {
		return false, nil
	}
	rc := rcs[len(rcs)-1]
	version := appsutil.DeploymentVersionFor(rc)
	if version != dc.Status.LatestVersion {
		//e2e.Logf("deployment %s is not the latest version on DC: %d", rc.Name, version)
		return false, nil
	}

	status := rc.Annotations[appsv1.DeploymentStatusAnnotation]
	switch appsv1.DeploymentStatus(status) {
	case appsv1.DeploymentStatusFailed:
		if appsutil.IsDeploymentCancelled(rc) {
			return true, nil
		}
		reason := appsutil.DeploymentStatusReasonFor(rc)
		if reason == "deployer pod no longer exists" {
			return true, nil
		}
		return false, fmt.Errorf("deployment failed: %v", appsutil.DeploymentStatusReasonFor(rc))

	case appsv1.DeploymentStatusRunning, appsv1.DeploymentStatusComplete:
		e2e.Logf("deployment %s/%s reached state %q", rc.Namespace, rc.Name, status)
		return true, nil

	default:
		return false, nil
	}
}

func deploymentPreHookRetried(dc *appsv1.DeploymentConfig, rcs []*corev1.ReplicationController, pods []corev1.Pod) (bool, error) {
	var preHook *corev1.Pod
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

func deploymentImageTriggersResolved(expectTriggers int) func(dc *appsv1.DeploymentConfig, rcs []*corev1.ReplicationController, pods []corev1.Pod) (bool, error) {
	return func(dc *appsv1.DeploymentConfig, rcs []*corev1.ReplicationController, pods []corev1.Pod) (bool, error) {
		expect := 0
		for _, t := range dc.Spec.Triggers {
			if t.Type != appsv1.DeploymentTriggerOnImageChange {
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

func deploymentInfo(oc *exutil.CLI, name string) (*appsv1.DeploymentConfig, []*corev1.ReplicationController, []corev1.Pod, error) {
	dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	// get pods before RCs, so we see more RCs than pods.
	pods, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).List(metav1.ListOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	rcs, err := oc.KubeClient().CoreV1().ReplicationControllers(oc.Namespace()).List(metav1.ListOptions{
		LabelSelector: appsutil.ConfigSelector(name).String(),
	})
	if err != nil {
		return nil, nil, nil, err
	}

	deployments := make([]*corev1.ReplicationController, 0, len(rcs.Items))
	for i := range rcs.Items {
		deployments = append(deployments, &rcs.Items[i])
	}

	sort.Sort(appsutil.ByLatestVersionAsc(deployments))

	return dc, deployments, pods.Items, nil
}

type deploymentConditionFunc func(dc *appsv1.DeploymentConfig, rcs []*corev1.ReplicationController, pods []corev1.Pod) (bool, error)

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
		config, err := oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return appsutil.HasSynced(config, generation), nil
	})
}

// waitForDeployerToComplete waits till the replication controller is created for a given
// rollout and then wait till the deployer pod finish. Then scrubs the deployer logs and
// return it.
func waitForDeployerToComplete(oc *exutil.CLI, name string, timeout time.Duration) (string, error) {
	watcher, err := oc.KubeClient().CoreV1().ReplicationControllers(oc.Namespace()).Watch(metav1.ListOptions{FieldSelector: fields.Everything().String()})
	if err != nil {
		return "", err
	}
	defer watcher.Stop()
	var rc *corev1.ReplicationController
	if _, err := watch.Until(timeout, watcher, func(e watch.Event) (bool, error) {
		if e.Type == watch.Error {
			return false, fmt.Errorf("error while waiting for replication controller: %v", e.Object)
		}
		if e.Type == watch.Added || e.Type == watch.Modified {
			if newRC, ok := e.Object.(*corev1.ReplicationController); ok && newRC.Name == name {
				rc = newRC
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return "", err
	}
	podName := appsutil.DeployerPodNameForDeployment(rc.Name)
	if err := deploylog.WaitForRunningDeployerPod(oc.KubeClient().CoreV1(), rc, timeout); err != nil {
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
	return !reflect.DeepEqual(old, metav1.GetControllerOf(controllee)), nil
}

func controllerRefChangeCondition(old *metav1.OwnerReference) func(controllee metav1.Object) (bool, error) {
	return func(controllee metav1.Object) (bool, error) {
		return isControllerRefChange(controllee, old)
	}
}

func rCConditionFromMeta(condition func(metav1.Object) (bool, error)) func(rc *corev1.ReplicationController) (bool, error) {
	return func(rc *corev1.ReplicationController) (bool, error) {
		return condition(rc)
	}
}

func waitForPodModification(oc *exutil.CLI, namespace string, name string, timeout time.Duration, resourceVersion string, condition func(pod *corev1.Pod) (bool, error)) (*corev1.Pod, error) {
	watcher, err := oc.KubeClient().CoreV1().Pods(namespace).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: name, ResourceVersion: resourceVersion}))
	if err != nil {
		return nil, err
	}

	event, err := watch.Until(timeout, watcher, func(event watch.Event) (bool, error) {
		if event.Type != watch.Modified && (resourceVersion == "" && event.Type != watch.Added) {
			return true, fmt.Errorf("different kind of event appeared while waiting for Pod modification: event: %#v", event)
		}
		return condition(event.Object.(*corev1.Pod))
	})
	if err != nil {
		return nil, err
	}
	return event.Object.(*corev1.Pod), nil
}

func waitForRCModification(oc *exutil.CLI, namespace string, name string, timeout time.Duration, resourceVersion string, condition func(rc *corev1.ReplicationController) (bool, error)) (*corev1.ReplicationController, error) {
	watcher, err := oc.KubeClient().CoreV1().ReplicationControllers(namespace).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: name, ResourceVersion: resourceVersion}))
	if err != nil {
		return nil, err
	}

	event, err := watch.Until(timeout, watcher, func(event watch.Event) (bool, error) {
		if event.Type != watch.Modified && (resourceVersion == "" && event.Type != watch.Added) {
			return true, fmt.Errorf("different kind of event appeared while waiting for RC modification: event: %#v", event)
		}
		return condition(event.Object.(*corev1.ReplicationController))
	})
	if err != nil {
		return nil, err
	}
	if event.Type != watch.Modified {
		return nil, fmt.Errorf("waiting for RC modification failed: event: %v", event)
	}
	return event.Object.(*corev1.ReplicationController), nil
}

func waitForDCModification(oc *exutil.CLI, namespace string, name string, timeout time.Duration, resourceVersion string, condition func(rc *appsv1.DeploymentConfig) (bool, error)) (*appsv1.DeploymentConfig, error) {
	watcher, err := oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: name, ResourceVersion: resourceVersion}))
	if err != nil {
		return nil, err
	}

	event, err := watch.Until(timeout, watcher, func(event watch.Event) (bool, error) {
		if event.Type != watch.Modified && (resourceVersion == "" && event.Type != watch.Added) {
			return true, fmt.Errorf("different kind of event appeared while waiting for DC modification: event: %#v", event)
		}
		return condition(event.Object.(*appsv1.DeploymentConfig))
	})
	if err != nil {
		return nil, err
	}
	return event.Object.(*appsv1.DeploymentConfig), nil
}

func createDeploymentConfig(oc *exutil.CLI, fixture string) (*appsv1.DeploymentConfig, error) {
	dcFixture, err := readDCFixture(fixture)
	if err != nil {
		return nil, err
	}
	dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Create(dcFixture)
	if err != nil {
		return nil, err
	}
	var pollErr error
	err = wait.PollImmediate(1*time.Second, 1*time.Minute, func() (bool, error) {
		dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Get(dc.Name, metav1.GetOptions{})
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

	for _, pod := range pods {
		if _, ok := pod.Labels[appsv1.DeployerPodForDeploymentLabel]; ok {
			continue
		}

		out, err := oc.Run("get").Args("pod/"+pod.Name, "-o", "yaml").Output()
		if err != nil {
			e2e.Logf("Error getting pod %s: %v", pod.Name, err)
			return
		}
		e2e.Logf("\n%s\n", out)
	}
	out, err = oc.Run("get").Args("istag", "-o", "wide").Output()
	if err != nil {
		e2e.Logf("Error getting image stream tags: %v", err)
	}
	e2e.Logf("\n%s\n", out)

}

func failureTrapForDetachedRCs(oc *exutil.CLI, dcName string, failed bool) {
	if !failed {
		return
	}
	kclient := oc.KubeClient()
	requirement, err := labels.NewRequirement(appsv1.DeploymentConfigAnnotation, selection.NotEquals, []string{dcName})
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
		out, err := oc.Run("get").Args("rc", "-o", "yaml", "-l", fmt.Sprintf("%s!=%s", appsv1.DeploymentConfigAnnotation, dcName)).Output()
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
	ref := metav1.GetControllerOf(controllee)
	deploymentConfigControllerRefKind := appsv1.GroupVersion.WithKind("DeploymentConfig")
	return ref != nil &&
		ref.UID == dc.GetUID() &&
		ref.APIVersion == deploymentConfigControllerRefKind.GroupVersion().String() &&
		ref.Kind == deploymentConfigControllerRefKind.Kind &&
		ref.Name == dc.GetName()
}

func readDCFixture(path string) (*appsv1.DeploymentConfig, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content, err := kyaml.ToJSON(data)
	if err != nil {
		return nil, err
	}
	appsScheme := runtime.NewScheme()
	appsCodecs := serializer.NewCodecFactory(appsScheme)
	appsv1.AddToScheme(appsScheme)
	obj, err := runtime.Decode(appsCodecs.UniversalDecoder(appsv1.GroupVersion), content)
	if err != nil {
		return nil, err
	}
	return obj.(*appsv1.DeploymentConfig), err
}

type deployerPodInvariantChecker struct {
	ctx       context.Context
	wg        sync.WaitGroup
	namespace string
	client    kubernetes.Interface
	cache     map[string][]*corev1.Pod
}

func NewDeployerPodInvariantChecker(namespace string, client kubernetes.Interface) *deployerPodInvariantChecker {
	return &deployerPodInvariantChecker{
		namespace: namespace,
		client:    client,
		cache:     make(map[string][]*corev1.Pod),
	}
}

func (d *deployerPodInvariantChecker) getCacheKey(pod *corev1.Pod) string {
	dcName, found := pod.Annotations[appsv1.DeploymentConfigAnnotation]
	o.Expect(found).To(o.BeTrue(), fmt.Sprintf("internal error - deployment is missing %q annotation\npod: %#v", appsv1.DeploymentConfigAnnotation,
		pod))
	o.Expect(dcName).NotTo(o.BeEmpty())

	return fmt.Sprintf("%s/%s", pod.Namespace, dcName)
}
func (d *deployerPodInvariantChecker) getPodIndex(list []*corev1.Pod, pod *corev1.Pod) int {
	for i, p := range list {
		if p.Name == pod.Name && p.Namespace == pod.Namespace {
			// Internal check
			o.Expect(p.UID).To(o.Equal(pod.UID))
			return i
		}
	}

	// Internal check
	o.Expect(fmt.Errorf("couldn't find pod %#v \n\n in list %#v", pod, list)).NotTo(o.HaveOccurred())
	return -1
}

func (d *deployerPodInvariantChecker) checkInvariants(dc string, pods []*corev1.Pod) {
	var unterminatedPods []*corev1.Pod
	for _, pod := range pods {
		if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
			unterminatedPods = append(unterminatedPods, pod)
		}
	}

	// INVARIANT: There can be no more than one unterminated deployer pod present
	message := fmt.Sprintf("Deployer pod invariant broken! More than one unterminated deployer pod exists for DC %s!", dc)
	o.Expect(len(unterminatedPods)).To(o.BeNumerically("<=", 1), spew.Sprintf(`%v: %s
		List of unterminated pods: %#+v
	`, time.Now(), message, unterminatedPods))
}

func (d *deployerPodInvariantChecker) AddPod(pod *corev1.Pod) {
	key := d.getCacheKey(pod)
	d.cache[key] = append(d.cache[key], pod)

	d.checkInvariants(key, d.cache[key])
}

func (d *deployerPodInvariantChecker) RemovePod(pod *corev1.Pod) {
	key := d.getCacheKey(pod)
	index := d.getPodIndex(d.cache[key], pod)

	d.cache[key] = append(d.cache[key][:index], d.cache[key][index+1:]...)

	d.checkInvariants(key, d.cache[key])
}

func (d *deployerPodInvariantChecker) UpdatePod(pod *corev1.Pod) {
	key := d.getCacheKey(pod)
	index := d.getPodIndex(d.cache[key], pod)

	// Check for sanity.
	// This is not paranoid; kubelet has already been broken this way:
	// https://github.com/openshift/origin/issues/17011
	oldPhase := d.cache[key][index].Status.Phase
	oldPhaseIsTerminated := oldPhase == corev1.PodSucceeded || oldPhase == corev1.PodFailed
	o.Expect(oldPhaseIsTerminated && pod.Status.Phase != oldPhase).To(o.BeFalse(),
		fmt.Sprintf("%v: detected deployer pod transition from terminated phase: %q -> %q", time.Now(), oldPhase, pod.Status.Phase))

	d.cache[key][index] = pod

	d.checkInvariants(key, d.cache[key])
}

func (d *deployerPodInvariantChecker) doChecking() {
	defer g.GinkgoRecover()

	watcher, err := d.client.CoreV1().Pods(d.namespace).Watch(metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	defer d.wg.Done()
	defer watcher.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case event := <-watcher.ResultChan():
			t := event.Type
			if t != watch.Added && t != watch.Modified && t != watch.Deleted {
				o.Expect(fmt.Errorf("unexpected event: %#v", event)).NotTo(o.HaveOccurred())
			}
			pod := event.Object.(*corev1.Pod)
			if !strings.HasSuffix(pod.Name, "-deploy") {
				continue
			}

			switch t {
			case watch.Added:
				d.AddPod(pod)
			case watch.Modified:
				d.UpdatePod(pod)
			case watch.Deleted:
				d.RemovePod(pod)
			}
		}
	}
}

func (d *deployerPodInvariantChecker) Start(ctx context.Context) {
	d.ctx = ctx
	go d.doChecking()
	d.wg.Add(1)
}

func (d *deployerPodInvariantChecker) Wait() {
	d.wg.Wait()
}
