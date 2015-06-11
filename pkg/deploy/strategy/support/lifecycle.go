package support

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/wait"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	namer "github.com/openshift/origin/pkg/util/namer"
)

// HookExecutor executes a deployment lifecycle hook.
type HookExecutor struct {
	// PodClient provides access to pods.
	PodClient HookExecutorPodClient
}

// Execute executes hook in the context of deployment. The label is used to
// distinguish the kind of hook (e.g. pre, post).
func (e *HookExecutor) Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error {
	var err error
	switch {
	case hook.ExecNewPod != nil:
		err = e.executeExecNewPod(hook, deployment, label)
	}

	if err == nil {
		return nil
	}

	// Retry failures are treated the same as Abort.
	switch hook.FailurePolicy {
	case deployapi.LifecycleHookFailurePolicyAbort, deployapi.LifecycleHookFailurePolicyRetry:
		return fmt.Errorf("Hook failed, aborting: %s", err)
	case deployapi.LifecycleHookFailurePolicyIgnore:
		glog.Infof("Hook failed, ignoring: %s", err)
		return nil
	default:
		return err
	}
}

// executeExecNewPod executes a ExecNewPod hook by creating a new pod based on
// the hook parameters and deployment. The pod is then synchronously watched
// until the pod completes, and if the pod failed, an error is returned.
//
// The hook pod inherits the following from the container the hook refers to:
//
//   * Environment (hook keys take precedence)
//   * Working directory
//   * Resources
func (e *HookExecutor) executeExecNewPod(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error {
	// Build a pod spec from the hook config and deployment
	podSpec, err := makeHookPod(hook, deployment, label)
	if err != nil {
		return err
	}

	// Try to create the pod.
	pod, err := e.PodClient.CreatePod(deployment.Namespace, podSpec)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return fmt.Errorf("couldn't create lifecycle pod for %s: %v", deployutil.LabelForDeployment(deployment), err)
		}
	} else {
		glog.V(0).Infof("Created lifecycle pod %s for deployment %s", pod.Name, deployutil.LabelForDeployment(deployment))
	}

	// Wait for the pod to finish.
	nextPod := e.PodClient.PodWatch(pod.Namespace, pod.Name, pod.ResourceVersion)
	glog.V(0).Infof("Waiting for hook pod %s/%s to complete", pod.Namespace, pod.Name)
	for {
		pod := nextPod()
		switch pod.Status.Phase {
		case kapi.PodSucceeded:
			return nil
		case kapi.PodFailed:
			return fmt.Errorf(pod.Status.Message)
		}
	}
}

// makeHookPod makes a pod spec from a hook and deployment.
func makeHookPod(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) (*kapi.Pod, error) {
	exec := hook.ExecNewPod
	var baseContainer *kapi.Container
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == exec.ContainerName {
			baseContainer = &container
			break
		}
	}
	if baseContainer == nil {
		return nil, fmt.Errorf("no container named '%s' found in deployment template", exec.ContainerName)
	}

	// Build a merged environment; hook environment takes precedence over base
	// container environment
	envMap := map[string]string{}
	mergedEnv := []kapi.EnvVar{}
	for _, env := range baseContainer.Env {
		envMap[env.Name] = env.Value
	}
	for _, env := range exec.Env {
		envMap[env.Name] = env.Value
	}
	for k, v := range envMap {
		mergedEnv = append(mergedEnv, kapi.EnvVar{Name: k, Value: v})
	}

	// Inherit resources from the base container
	resources := kapi.ResourceRequirements{}
	if err := kapi.Scheme.Convert(&baseContainer.Resources, &resources); err != nil {
		return nil, fmt.Errorf("couldn't clone ResourceRequirements: %v", err)
	}

	// Assigning to a variable since its address is required
	maxDeploymentDurationSeconds := deployapi.MaxDeploymentDurationSeconds

	// Let the kubelet manage retries if requested
	restartPolicy := kapi.RestartPolicyNever
	if hook.FailurePolicy == deployapi.LifecycleHookFailurePolicyRetry {
		restartPolicy = kapi.RestartPolicyOnFailure
	}

	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: namer.GetPodName(deployment.Name, label),
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: deployment.Name,
			},
			Labels: map[string]string{
				deployapi.DeployerPodForDeploymentLabel: deployment.Name,
			},
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:       "lifecycle",
					Image:      baseContainer.Image,
					Command:    exec.Command,
					WorkingDir: baseContainer.WorkingDir,
					Env:        mergedEnv,
					Resources:  resources,
				},
			},
			ActiveDeadlineSeconds: &maxDeploymentDurationSeconds,
			RestartPolicy:         restartPolicy,
		},
	}

	return pod, nil
}

// HookExecutorPodClient abstracts access to pods.
type HookExecutorPodClient interface {
	CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	PodWatch(namespace, name, resourceVersion string) func() *kapi.Pod
}

// HookExecutorPodClientImpl is a pluggable HookExecutorPodClient.
type HookExecutorPodClientImpl struct {
	CreatePodFunc func(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	PodWatchFunc  func(namespace, name, resourceVersion string) func() *kapi.Pod
}

func (i *HookExecutorPodClientImpl) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return i.CreatePodFunc(namespace, pod)
}

func (i *HookExecutorPodClientImpl) PodWatch(namespace, name, resourceVersion string) func() *kapi.Pod {
	return i.PodWatchFunc(namespace, name, resourceVersion)
}

// NewPodWatch creates a pod watching function which is backed by a
// FIFO/reflector pair. This avoids managing watches directly.
func NewPodWatch(client kclient.Interface, namespace, name, resourceVersion string) func() *kapi.Pod {
	fieldSelector, _ := fields.ParseSelector("metadata.name=" + name)
	podLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return client.Pods(namespace).List(labels.Everything(), fieldSelector)
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return client.Pods(namespace).Watch(labels.Everything(), fieldSelector, resourceVersion)
		},
	}
	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(podLW, &kapi.Pod{}, queue, 1*time.Minute).Run()

	return func() *kapi.Pod {
		obj := queue.Pop()
		return obj.(*kapi.Pod)
	}
}

func NewFirstContainerReady(kclient kclient.Interface, timeout time.Duration, interval time.Duration) *FirstContainerReady {
	return &FirstContainerReady{
		timeout:  timeout,
		interval: interval,
		podsForDeployment: func(deployment *kapi.ReplicationController) (*kapi.PodList, error) {
			selector := labels.Set(deployment.Spec.Selector).AsSelector()
			return kclient.Pods(deployment.Namespace).List(selector, fields.Everything())
		},
		getPodStore: func(namespace, name string) (cache.Store, chan struct{}) {
			sel, _ := fields.ParseSelector("metadata.name=" + name)
			store := cache.NewStore(cache.MetaNamespaceKeyFunc)
			lw := &deployutil.ListWatcherImpl{
				ListFunc: func() (runtime.Object, error) {
					return kclient.Pods(namespace).List(labels.Everything(), sel)
				},
				WatchFunc: func(resourceVersion string) (watch.Interface, error) {
					return kclient.Pods(namespace).Watch(labels.Everything(), sel, resourceVersion)
				},
			}
			stop := make(chan struct{})
			cache.NewReflector(lw, &kapi.Pod{}, store, 10*time.Second).RunUntil(stop)
			return store, stop
		},
	}
}

type FirstContainerReady struct {
	podsForDeployment func(*kapi.ReplicationController) (*kapi.PodList, error)
	getPodStore       func(namespace, name string) (cache.Store, chan struct{})
	timeout           time.Duration
	interval          time.Duration
}

func (c *FirstContainerReady) Accept(deployment *kapi.ReplicationController) error {
	// For now, only validate the first replica.
	if deployment.Spec.Replicas != 1 {
		glog.Infof("automatically accepting deployment %s with %d replicas", deployutil.LabelForDeployment(deployment), deployment.Spec.Replicas)
		return nil
	}

	// Try and find the pod for the deployment.
	pods, err := c.podsForDeployment(deployment)
	if err != nil {
		return fmt.Errorf("couldn't get pods for deployment %s: %v", deployutil.LabelForDeployment(deployment), err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for deployment %s", deployutil.LabelForDeployment(deployment))
	}

	// If we found multiple, use the first one and log a warning.
	// TODO: should finding multiple be an error?
	pod := &pods.Items[0]
	if len(pods.Items) > 1 {
		glog.Infof("Warning: more than one pod for deployment %s; basing canary check on the first pod '%s'", deployutil.LabelForDeployment(deployment), pod.Name)
	}

	// Make a pod store to poll and ensure it gets cleaned up.
	podStore, stopStore := c.getPodStore(pod.Namespace, pod.Name)
	defer close(stopStore)

	// Track container readiness based on those defined in the spec.
	observedContainers := map[string]bool{}
	for _, container := range pod.Spec.Containers {
		observedContainers[container.Name] = false
	}

	// Start checking for pod updates.
	glog.V(0).Infof("Waiting for pod %s/%s container readiness", pod.Namespace, pod.Name)
	return wait.Poll(c.interval, c.timeout, func() (done bool, err error) {
		// Get the latest state of the pod.
		obj, exists, err := podStore.Get(pod)
		// Try again later on error or if the pod isn't available yet.
		if err != nil {
			glog.V(0).Infof("Error getting pod %s/%s to inspect container readiness: %v", pod.Namespace, pod.Name, err)
			return false, nil
		}
		if !exists {
			glog.V(0).Infof("Couldn't find pod %s/%s to inspect container readiness", pod.Namespace, pod.Name)
			return false, nil
		}
		// New pod state is available; update the observed ready status of any
		// containers.
		updatedPod := obj.(*kapi.Pod)
		for _, status := range updatedPod.Status.ContainerStatuses {
			// Ignore any containers which aren't defined in the deployment spec.
			if _, known := observedContainers[status.Name]; !known {
				glog.V(0).Infof("Ignoring readiness of container %s in pod %s/%s because it's not present in the pod spec", status.Name, pod.Namespace, pod.Name)
				continue
			}
			// The status of the container could be transient; we only care if it
			// was ever ready. If it was ready and then became not ready, we
			// consider it ready.
			if status.Ready {
				observedContainers[status.Name] = true
			}
		}
		// Check whether all containers have been observed as ready.
		allReady := true
		for _, ready := range observedContainers {
			if !ready {
				allReady = false
				break
			}
		}
		// If all containers have been ready once, return success.
		if allReady {
			glog.V(0).Infof("All containers ready for %s/%s", pod.Namespace, pod.Name)
			return true, nil
		}
		// Otherwise, try again later.
		glog.V(4).Infof("Still waiting for pod %s/%s container readiness; observed statuses: #%v", pod.Namespace, pod.Name, observedContainers)
		return false, nil
	})
}
