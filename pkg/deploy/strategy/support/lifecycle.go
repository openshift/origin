package support

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	namer "github.com/openshift/origin/pkg/util/namer"
)

const HookContainerName = "lifecycle"

// HookExecutor executes a deployment lifecycle hook.
type HookExecutor struct {
	// podClient provides access to pods.
	podClient HookExecutorPodClient
	// podLogDestination is where hook pod logs should be written to.
	podLogDestination io.Writer
	// podLogStream provides a reader for a pod's logs.
	podLogStream func(namespace, name string, opts *kapi.PodLogOptions) (io.ReadCloser, error)
}

// NewHookExecutor makes a HookExecutor from a client.
func NewHookExecutor(client kclient.Interface, podLogDestination io.Writer) *HookExecutor {
	return &HookExecutor{
		podClient: &HookExecutorPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return client.Pods(namespace).Create(pod)
			},
			PodWatchFunc: func(namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod {
				return NewPodWatch(client, namespace, name, resourceVersion, stopChannel)
			},
		},
		podLogStream: func(namespace, name string, opts *kapi.PodLogOptions) (io.ReadCloser, error) {
			req, err := client.PodLogs(namespace).Get(name, opts)
			if err != nil {
				return nil, err
			}
			return req.Stream()
		},
		podLogDestination: podLogDestination,
	}
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
	pod, err := e.podClient.CreatePod(deployment.Namespace, podSpec)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return fmt.Errorf("couldn't create lifecycle pod for %s: %v", deployutil.LabelForDeployment(deployment), err)
		}
	} else {
		glog.V(0).Infof("Created lifecycle pod %s/%s for deployment %s", pod.Namespace, pod.Name, deployutil.LabelForDeployment(deployment))
	}

	stopChannel := make(chan struct{})
	defer close(stopChannel)
	nextPod := e.podClient.PodWatch(pod.Namespace, pod.Name, pod.ResourceVersion, stopChannel)

	// Wait for the hook pod to reach a terminal phase. Start reading logs as
	// soon as the pod enters a usable phase.
	var updatedPod *kapi.Pod
	var once sync.Once
	wg := &sync.WaitGroup{}
	wg.Add(1)
	glog.V(0).Infof("Watching logs for hook pod %s/%s while awaiting completion", pod.Namespace, pod.Name)
waitLoop:
	for {
		updatedPod = nextPod()
		switch updatedPod.Status.Phase {
		case kapi.PodRunning:
			go once.Do(func() { e.readPodLogs(pod, wg) })
		case kapi.PodSucceeded, kapi.PodFailed:
			go once.Do(func() { e.readPodLogs(pod, wg) })
			break waitLoop
		}
	}
	// The pod is finished, wait for all logs to be consumed before returning.
	wg.Wait()
	if updatedPod.Status.Phase == kapi.PodFailed {
		return fmt.Errorf(updatedPod.Status.Message)
	}
	return nil
}

// readPodLogs streams logs from pod to podLogDestination. It signals wg when
// done.
func (e *HookExecutor) readPodLogs(pod *kapi.Pod, wg *sync.WaitGroup) {
	defer wg.Done()
	opts := &kapi.PodLogOptions{
		Container:  HookContainerName,
		Follow:     true,
		Timestamps: false,
	}
	logStream, err := e.podLogStream(pod.Namespace, pod.Name, opts)
	if err != nil || logStream == nil {
		glog.V(0).Infof("Warning: couldn't get log stream for lifecycle pod %s/%s: %s", pod.Namespace, pod.Name, err)
		return
	}
	// Read logs.
	defer logStream.Close()
	written, err := io.Copy(e.podLogDestination, logStream)
	if err != nil {
		glog.V(0).Infof("Finished reading logs for hook pod %s/%s (%d bytes): %s", pod.Namespace, pod.Name, written, err)
	} else {
		glog.V(0).Infof("Finished reading logs for hook pod %s/%s", pod.Namespace, pod.Name)
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
	mergedEnv = append(mergedEnv, kapi.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAME", Value: deployment.Name})
	mergedEnv = append(mergedEnv, kapi.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAMESPACE", Value: deployment.Namespace})

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

	// Transfer any requested volumes to the hook pod.
	volumes := []kapi.Volume{}
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		for _, name := range exec.Volumes {
			if volume.Name == name {
				volumes = append(volumes, volume)
			}
		}
	}

	// Transfer image pull secrets from the pod spec.
	imagePullSecrets := []kapi.LocalObjectReference{}
	for _, pullSecret := range deployment.Spec.Template.Spec.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, kapi.LocalObjectReference{Name: pullSecret.Name})
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
					Name:       HookContainerName,
					Image:      baseContainer.Image,
					Command:    exec.Command,
					WorkingDir: baseContainer.WorkingDir,
					Env:        mergedEnv,
					Resources:  resources,
				},
			},
			Volumes:               volumes,
			ActiveDeadlineSeconds: &maxDeploymentDurationSeconds,
			// Setting the node selector on the hook pod so that it is created
			// on the same set of nodes as the deployment pods.
			NodeSelector:     deployment.Spec.Template.Spec.NodeSelector,
			RestartPolicy:    restartPolicy,
			ImagePullSecrets: imagePullSecrets,
		},
	}

	return pod, nil
}

// HookExecutorPodClient abstracts access to pods.
type HookExecutorPodClient interface {
	CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	PodWatch(namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod
}

// HookExecutorPodClientImpl is a pluggable HookExecutorPodClient.
type HookExecutorPodClientImpl struct {
	CreatePodFunc func(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	PodWatchFunc  func(namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod
}

func (i *HookExecutorPodClientImpl) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return i.CreatePodFunc(namespace, pod)
}

func (i *HookExecutorPodClientImpl) PodWatch(namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod {
	return i.PodWatchFunc(namespace, name, resourceVersion, stopChannel)
}

// NewPodWatch creates a pod watching function which is backed by a
// FIFO/reflector pair. This avoids managing watches directly.
// A stop channel to close the watch's reflector is also returned.
// It is the caller's responsibility to defer closing the stop channel to prevent leaking resources.
func NewPodWatch(client kclient.Interface, namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name)
	podLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return client.Pods(namespace).List(labels.Everything(), fieldSelector)
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return client.Pods(namespace).Watch(labels.Everything(), fieldSelector, resourceVersion)
		},
	}

	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(podLW, &kapi.Pod{}, queue, 1*time.Minute).RunUntil(stopChannel)

	return func() *kapi.Pod {
		obj := queue.Pop()
		return obj.(*kapi.Pod)
	}
}

// NewAcceptNewlyObservedReadyPods makes a new AcceptNewlyObservedReadyPods
// from a real client.
func NewAcceptNewlyObservedReadyPods(kclient kclient.Interface, timeout time.Duration, interval time.Duration) *AcceptNewlyObservedReadyPods {
	return &AcceptNewlyObservedReadyPods{
		timeout:      timeout,
		interval:     interval,
		acceptedPods: sets.NewString(),
		getDeploymentPodStore: func(deployment *kapi.ReplicationController) (cache.Store, chan struct{}) {
			selector := labels.Set(deployment.Spec.Selector).AsSelector()
			store := cache.NewStore(cache.MetaNamespaceKeyFunc)
			lw := &deployutil.ListWatcherImpl{
				ListFunc: func() (runtime.Object, error) {
					return kclient.Pods(deployment.Namespace).List(selector, fields.Everything())
				},
				WatchFunc: func(resourceVersion string) (watch.Interface, error) {
					return kclient.Pods(deployment.Namespace).Watch(selector, fields.Everything(), resourceVersion)
				},
			}
			stop := make(chan struct{})
			cache.NewReflector(lw, &kapi.Pod{}, store, 10*time.Second).RunUntil(stop)
			return store, stop
		},
	}
}

// AcceptNewlyObservedReadyPods is a kubectl.UpdateAcceptor which will accept
// a deployment if all the containers in all of the pods for the deployment
// are observed to be ready at least once.
//
// AcceptNewlyObservedReadyPods keeps track of the pods it has accepted for a
// deployment so that the acceptor can be reused across multiple batches of
// updates to a single controller. For example, if during the first acceptance
// call the deployment has 3 pods, the acceptor will validate those 3 pods. If
// the same acceptor instance is used again for the same deployment which now
// has 6 pods, only the latest 3 pods will be considered for acceptance. The
// status of the original 3 pods becomes irrelevant.
//
// Note that this struct is stateful and intended for use with a single
// deployment and should be discarded and recreated between deployments.
type AcceptNewlyObservedReadyPods struct {
	// getDeploymentPodStore should return a Store containing all the pods for
	// the named deployment, and a channel to stop whatever process is feeding
	// the store.
	getDeploymentPodStore func(deployment *kapi.ReplicationController) (cache.Store, chan struct{})
	// timeout is how long to wait for pod readiness.
	timeout time.Duration
	// interval is how often to check for pod readiness
	interval time.Duration
	// acceptedPods keeps track of pods which have been previously accepted for
	// a deployment.
	acceptedPods sets.String
}

// Accept implements UpdateAcceptor.
func (c *AcceptNewlyObservedReadyPods) Accept(deployment *kapi.ReplicationController) error {
	// Make a pod store to poll and ensure it gets cleaned up.
	podStore, stopStore := c.getDeploymentPodStore(deployment)
	defer close(stopStore)

	// Start checking for pod updates.
	glog.V(0).Infof("Waiting %.f seconds for pods owned by deployment %q to become ready (checking every %.f seconds; %d pods previously accepted)", c.timeout.Seconds(), deployutil.LabelForDeployment(deployment), c.interval.Seconds(), c.acceptedPods.Len())
	err := wait.Poll(c.interval, c.timeout, func() (done bool, err error) {
		// Check for pod readiness.
		unready := sets.NewString()
		for _, obj := range podStore.List() {
			pod := obj.(*kapi.Pod)
			// Skip previously accepted pods; we only want to verify newly observed
			// and unaccepted pods.
			if c.acceptedPods.Has(pod.Name) {
				continue
			}
			if kapi.IsPodReady(pod) {
				// If the pod is ready, track it as accepted.
				c.acceptedPods.Insert(pod.Name)
			} else {
				// Otherwise, track it as unready.
				unready.Insert(pod.Name)
			}
		}
		// Check to see if we're done.
		if unready.Len() == 0 {
			glog.V(0).Infof("All pods ready for %s", deployutil.LabelForDeployment(deployment))
			return true, nil
		}
		// Otherwise, try again later.
		glog.V(4).Infof("Still waiting for %d pods to become ready for deployment %s", unready.Len(), deployutil.LabelForDeployment(deployment))
		return false, nil
	})

	// Handle acceptance failure.
	if err != nil {
		if err == wait.ErrWaitTimeout {
			return fmt.Errorf("pods for deployment %q took longer than %.f seconds to become ready", deployutil.LabelForDeployment(deployment), c.timeout.Seconds())
		}
		return fmt.Errorf("pod readiness check failed for deployment %q: %v", deployutil.LabelForDeployment(deployment), err)
	}
	return nil
}
