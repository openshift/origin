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
	utilerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/util"
	namer "github.com/openshift/origin/pkg/util/namer"
)

const HookContainerName = "lifecycle"

// HookExecutor executes a deployment lifecycle hook.
type HookExecutor struct {
	// podClient provides access to pods.
	podClient HookExecutorPodClient
	// tags allows setting image stream tags
	tags client.ImageStreamTagsNamespacer
	// out is where hook pod logs should be written to.
	out io.Writer
	// podLogStream provides a reader for a pod's logs.
	podLogStream func(namespace, name string, opts *kapi.PodLogOptions) (io.ReadCloser, error)
	// decoder is used for encoding/decoding.
	decoder runtime.Decoder
}

// NewHookExecutor makes a HookExecutor from a client.
func NewHookExecutor(client kclient.PodsNamespacer, tags client.ImageStreamTagsNamespacer, out io.Writer, decoder runtime.Decoder) *HookExecutor {
	return &HookExecutor{
		tags: tags,
		podClient: &HookExecutorPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return client.Pods(namespace).Create(pod)
			},
			PodWatchFunc: func(namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod {
				return NewPodWatch(client, namespace, name, resourceVersion, stopChannel)
			},
		},
		podLogStream: func(namespace, name string, opts *kapi.PodLogOptions) (io.ReadCloser, error) {
			return client.Pods(namespace).GetLogs(name, opts).Stream()
		},
		out:     out,
		decoder: decoder,
	}
}

// Execute executes hook in the context of deployment. The suffix is used to
// distinguish the kind of hook (e.g. pre, post).
func (e *HookExecutor) Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, suffix, label string) error {
	var err error
	switch {
	case len(hook.TagImages) > 0:
		err = e.tagImages(hook, deployment, suffix, label)
	case hook.ExecNewPod != nil:
		err = e.executeExecNewPod(hook, deployment, suffix, label)
	}

	if err == nil {
		return nil
	}

	// Retry failures are treated the same as Abort.
	switch hook.FailurePolicy {
	case deployapi.LifecycleHookFailurePolicyAbort, deployapi.LifecycleHookFailurePolicyRetry:
		return fmt.Errorf("%s hook failed, aborting deployment: %s", label, err)
	case deployapi.LifecycleHookFailurePolicyIgnore:
		fmt.Fprintf(e.out, "warning: %s hook failed, deployment will continue: %v\n", label, err)
		return nil
	default:
		return err
	}
}

func findContainerImage(rc *kapi.ReplicationController, containerName string) (string, bool) {
	if rc.Spec.Template == nil {
		return "", false
	}
	for _, container := range rc.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			return container.Image, true
		}
	}
	return "", false
}

// tagImages tags images from the deployment
func (e *HookExecutor) tagImages(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, suffix, label string) error {
	var errs []error
	for _, action := range hook.TagImages {
		value, ok := findContainerImage(deployment, action.ContainerName)
		if !ok {
			errs = append(errs, fmt.Errorf("unable to find image for container %q, container could not be found", action.ContainerName))
			continue
		}
		namespace := action.To.Namespace
		if len(namespace) == 0 {
			namespace = deployment.Namespace
		}
		if _, err := e.tags.ImageStreamTags(namespace).Update(&imageapi.ImageStreamTag{
			ObjectMeta: kapi.ObjectMeta{
				Name:      action.To.Name,
				Namespace: namespace,
			},
			Tag: &imageapi.TagReference{
				From: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: value,
				},
			},
		}); err != nil {
			errs = append(errs, err)
			continue
		}
		fmt.Fprintf(e.out, "--> %s: Tagged %q into %s/%s\n", label, value, action.To.Namespace, action.To.Name)
	}

	return utilerrors.NewAggregate(errs)
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
func (e *HookExecutor) executeExecNewPod(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, suffix, label string) error {
	config, err := deployutil.DecodeDeploymentConfig(deployment, e.decoder)
	if err != nil {
		return err
	}

	// Build a pod spec from the hook config and deployment
	podSpec, err := makeHookPod(hook, deployment, &config.Spec.Strategy, suffix)
	if err != nil {
		return err
	}

	// Track whether the pod has already run to completion and avoid showing logs
	// or the Success message twice.
	completed, created := false, false

	// Try to create the pod.
	pod, err := e.podClient.CreatePod(deployment.Namespace, podSpec)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return fmt.Errorf("couldn't create lifecycle pod for %s: %v", deployment.Name, err)
		}
		completed = true
		pod = podSpec
		pod.Namespace = deployment.Namespace
	} else {
		created = true
		fmt.Fprintf(e.out, "--> %s: Running hook pod ...\n", label)
	}

	stopChannel := make(chan struct{})
	defer close(stopChannel)
	nextPod := e.podClient.PodWatch(pod.Namespace, pod.Name, pod.ResourceVersion, stopChannel)

	// Wait for the hook pod to reach a terminal phase. Start reading logs as
	// soon as the pod enters a usable phase.
	var updatedPod *kapi.Pod
	wg := &sync.WaitGroup{}
	wg.Add(1)
	restarts := int32(0)
	alreadyRead := false
waitLoop:
	for {
		updatedPod = nextPod()
		switch updatedPod.Status.Phase {
		case kapi.PodRunning:
			completed = false

			// We should read only the first time or in any container restart when we want to retry.
			canRetry, restartCount := canRetryReading(updatedPod, restarts)
			if alreadyRead && !canRetry {
				break
			}
			// The hook container has restarted; we need to notify that we are retrying in the logs.
			// TODO: Maybe log the container id
			if restarts != restartCount {
				wg.Add(1)
				restarts = restartCount
				fmt.Fprintf(e.out, "--> %s: Retrying hook pod (retry #%d)\n", label, restartCount)
			}
			alreadyRead = true
			go e.readPodLogs(pod, wg)

		case kapi.PodSucceeded, kapi.PodFailed:
			if completed {
				if updatedPod.Status.Phase == kapi.PodSucceeded {
					fmt.Fprintf(e.out, "--> %s: Hook pod already succeeded\n", label)
				}
				wg.Done()
				break waitLoop
			}
			if !created {
				fmt.Fprintf(e.out, "--> %s: Hook pod is already running ...\n", label)
			}
			if !alreadyRead {
				go e.readPodLogs(pod, wg)
			}
			break waitLoop
		default:
			completed = false
		}
	}
	// The pod is finished, wait for all logs to be consumed before returning.
	wg.Wait()
	if updatedPod.Status.Phase == kapi.PodFailed {
		return fmt.Errorf(updatedPod.Status.Message)
	}
	// Only show this message if we created the pod ourselves, or we saw
	// the pod in a running or pending state.
	if !completed {
		fmt.Fprintf(e.out, "--> %s: Success\n", label)
	}
	return nil
}

// readPodLogs streams logs from pod to out. It signals wg when
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
		fmt.Fprintf(e.out, "warning: Unable to retrieve hook logs from %s: %v\n", pod.Name, err)
		return
	}
	// Read logs.
	defer logStream.Close()
	if _, err := io.Copy(e.out, logStream); err != nil {
		fmt.Fprintf(e.out, "\nwarning: Unable to read all logs from %s, continuing: %v\n", pod.Name, err)
	}
}

// makeHookPod makes a pod spec from a hook and deployment.
func makeHookPod(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, strategy *deployapi.DeploymentStrategy, suffix string) (*kapi.Pod, error) {
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
	volumeNames := sets.NewString()
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		for _, name := range exec.Volumes {
			if volume.Name == name {
				volumes = append(volumes, volume)
				volumeNames.Insert(volume.Name)
			}
		}
	}
	// Transfer any volume mounts associated with transferred volumes.
	volumeMounts := []kapi.VolumeMount{}
	for _, mount := range baseContainer.VolumeMounts {
		if volumeNames.Has(mount.Name) {
			volumeMounts = append(volumeMounts, kapi.VolumeMount{
				Name:      mount.Name,
				ReadOnly:  mount.ReadOnly,
				MountPath: mount.MountPath,
			})
		}
	}

	// Transfer image pull secrets from the pod spec.
	imagePullSecrets := []kapi.LocalObjectReference{}
	for _, pullSecret := range deployment.Spec.Template.Spec.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, kapi.LocalObjectReference{Name: pullSecret.Name})
	}

	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: namer.GetPodName(deployment.Name, suffix),
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: deployment.Name,
			},
			Labels: map[string]string{
				deployapi.DeploymentPodTypeLabel:        suffix,
				deployapi.DeployerPodForDeploymentLabel: deployment.Name,
			},
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:         HookContainerName,
					Image:        baseContainer.Image,
					Command:      exec.Command,
					WorkingDir:   baseContainer.WorkingDir,
					Env:          mergedEnv,
					Resources:    resources,
					VolumeMounts: volumeMounts,
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

	util.MergeInto(pod.Labels, strategy.Labels, 0)
	util.MergeInto(pod.Annotations, strategy.Annotations, 0)

	return pod, nil
}

func canRetryReading(pod *kapi.Pod, restarts int32) (bool, int32) {
	if len(pod.Status.ContainerStatuses) == 0 {
		return false, int32(0)
	}
	restartCount := pod.Status.ContainerStatuses[0].RestartCount
	return pod.Spec.RestartPolicy == kapi.RestartPolicyOnFailure && restartCount > restarts, restartCount
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
func NewPodWatch(client kclient.PodsNamespacer, namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name)
	podLW := &cache.ListWatch{
		ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
			opts := kapi.ListOptions{FieldSelector: fieldSelector}
			return client.Pods(namespace).List(opts)
		},
		WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
			opts := kapi.ListOptions{FieldSelector: fieldSelector, ResourceVersion: options.ResourceVersion}
			return client.Pods(namespace).Watch(opts)
		},
	}

	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(podLW, &kapi.Pod{}, queue, 1*time.Minute).RunUntil(stopChannel)

	return func() *kapi.Pod {
		obj := cache.Pop(queue)
		return obj.(*kapi.Pod)
	}
}

// NewAcceptNewlyObservedReadyPods makes a new AcceptNewlyObservedReadyPods
// from a real client.
func NewAcceptNewlyObservedReadyPods(out io.Writer, kclient kclient.PodsNamespacer, timeout time.Duration, interval time.Duration) *AcceptNewlyObservedReadyPods {
	return &AcceptNewlyObservedReadyPods{
		out:          out,
		timeout:      timeout,
		interval:     interval,
		acceptedPods: sets.NewString(),
		getDeploymentPodStore: func(deployment *kapi.ReplicationController) (cache.Store, chan struct{}) {
			selector := labels.Set(deployment.Spec.Selector).AsSelector()
			store := cache.NewStore(cache.MetaNamespaceKeyFunc)
			lw := &cache.ListWatch{
				ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
					opts := kapi.ListOptions{LabelSelector: selector}
					return kclient.Pods(deployment.Namespace).List(opts)
				},
				WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
					opts := kapi.ListOptions{LabelSelector: selector, ResourceVersion: options.ResourceVersion}
					return kclient.Pods(deployment.Namespace).Watch(opts)
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
	out io.Writer
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
	if c.acceptedPods.Len() > 0 {
		fmt.Fprintf(c.out, "--> Waiting up to %s for pods in deployment %s to become ready (%d pods previously accepted)\n", c.timeout, deployment.Name, c.acceptedPods.Len())
	} else {
		fmt.Fprintf(c.out, "--> Waiting up to %s for pods in deployment %s to become ready\n", c.timeout, deployment.Name)
	}
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
			return true, nil
		}
		// Otherwise, try again later.
		glog.V(4).Infof("Still waiting for %d pods to become ready for deployment %s", unready.Len(), deployment.Name)
		return false, nil
	})

	// Handle acceptance failure.
	if err != nil {
		if err == wait.ErrWaitTimeout {
			return fmt.Errorf("pods for deployment %q took longer than %.f seconds to become ready", deployment.Name, c.timeout.Seconds())
		}
		return fmt.Errorf("pod readiness check failed for deployment %q: %v", deployment.Name, err)
	}
	return nil
}
