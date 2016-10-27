package support

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kdeployutil "k8s.io/kubernetes/pkg/controller/deployment/util"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	utilerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	strategyutil "github.com/openshift/origin/pkg/deploy/strategy/util"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/util"
	namer "github.com/openshift/origin/pkg/util/namer"
)

const HookContainerName = "lifecycle"

// HookExecutor executes a deployment lifecycle hook.
type HookExecutor struct {
	// pods provides client to pods
	pods kclient.PodsNamespacer
	// tags allows setting image stream tags
	tags client.ImageStreamTagsNamespacer
	// out is where hook pod logs should be written to.
	out io.Writer
	// decoder is used for encoding/decoding.
	decoder runtime.Decoder
	// recorder is used to emit events from hooks
	events kclient.EventNamespacer
	// getPodLogs knows how to get logs from a pod and is used for testing
	getPodLogs func(*kapi.Pod) (io.ReadCloser, error)
}

// NewHookExecutor makes a HookExecutor from a client.
func NewHookExecutor(pods kclient.PodsNamespacer, tags client.ImageStreamTagsNamespacer, events kclient.EventNamespacer, out io.Writer, decoder runtime.Decoder) *HookExecutor {
	executor := &HookExecutor{
		tags:    tags,
		pods:    pods,
		events:  events,
		out:     out,
		decoder: decoder,
	}
	executor.getPodLogs = func(pod *kapi.Pod) (io.ReadCloser, error) {
		opts := &kapi.PodLogOptions{
			Container:  HookContainerName,
			Follow:     true,
			Timestamps: false,
		}
		return executor.pods.Pods(pod.Namespace).GetLogs(pod.Name, opts).Stream()
	}
	return executor
}

// Execute executes hook in the context of deployment. The suffix is used to
// distinguish the kind of hook (e.g. pre, post).
func (e *HookExecutor) Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, suffix, label string) error {
	var err error
	switch {
	case len(hook.TagImages) > 0:
		tagEventMessages := []string{}
		for _, t := range hook.TagImages {
			image, ok := findContainerImage(deployment, t.ContainerName)
			if ok {
				tagEventMessages = append(tagEventMessages, fmt.Sprintf("image %q as %q", image, t.To.Name))
			}
		}
		strategyutil.RecordConfigEvent(e.events, deployment, e.decoder, kapi.EventTypeNormal, "Started",
			fmt.Sprintf("Running %s-hook (TagImages) %s for deployment %s/%s", label, strings.Join(tagEventMessages, ","), deployment.Namespace, deployment.Name))
		err = e.tagImages(hook, deployment, suffix, label)
	case hook.ExecNewPod != nil:
		strategyutil.RecordConfigEvent(e.events, deployment, e.decoder, kapi.EventTypeNormal, "Started",
			fmt.Sprintf("Running %s-hook (%q) for deployment %s/%s", label, strings.Join(hook.ExecNewPod.Command, " "), deployment.Namespace, deployment.Name))
		err = e.executeExecNewPod(hook, deployment, suffix, label)
	}

	if err == nil {
		strategyutil.RecordConfigEvent(e.events, deployment, e.decoder, kapi.EventTypeNormal, "Completed",
			fmt.Sprintf("The %s-hook for deployment %s/%s completed successfully", label, deployment.Namespace, deployment.Name))
		return nil
	}

	// Retry failures are treated the same as Abort.
	switch hook.FailurePolicy {
	case deployapi.LifecycleHookFailurePolicyAbort, deployapi.LifecycleHookFailurePolicyRetry:
		strategyutil.RecordConfigEvent(e.events, deployment, e.decoder, kapi.EventTypeWarning, "Failed",
			fmt.Sprintf("The %s-hook failed: %v, aborting deployment %s/%s", label, err, deployment.Namespace, deployment.Name))
		return fmt.Errorf("the %s hook failed: %v, aborting deployment: %s/%s", label, err, deployment.Namespace, deployment.Name)
	case deployapi.LifecycleHookFailurePolicyIgnore:
		strategyutil.RecordConfigEvent(e.events, deployment, e.decoder, kapi.EventTypeWarning, "Failed",
			fmt.Sprintf("The %s-hook failed: %v (ignore), deployment %s/%s will continue", label, err, deployment.Namespace, deployment.Name))
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

	deployerPod, err := e.pods.Pods(deployment.Namespace).Get(deployutil.DeployerPodNameForDeployment(deployment.Name))
	if err != nil {
		return err
	}

	// Build a pod spec from the hook config and deployment
	podSpec, err := makeHookPod(hook, deployment, deployerPod, &config.Spec.Strategy, suffix)
	if err != nil {
		return err
	}

	// Track whether the pod has already run to completion and avoid showing logs
	// or the Success message twice.
	completed, created := false, false

	// Try to create the pod.
	pod, err := e.pods.Pods(deployment.Namespace).Create(podSpec)
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
	nextPod := NewPodWatch(e.pods.Pods(pod.Namespace), pod.Namespace, pod.Name, pod.ResourceVersion, stopChannel)

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
		fmt.Fprintf(e.out, "--> %s: Failed\n", label)
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
	logStream, err := e.getPodLogs(pod)
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
func makeHookPod(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, deployerPod *kapi.Pod, strategy *deployapi.DeploymentStrategy, suffix string) (*kapi.Pod, error) {
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
	envMap := map[string]kapi.EnvVar{}
	mergedEnv := []kapi.EnvVar{}
	for _, env := range baseContainer.Env {
		envMap[env.Name] = env
	}
	for _, env := range exec.Env {
		envMap[env.Name] = env
	}
	for k, v := range envMap {
		mergedEnv = append(mergedEnv, kapi.EnvVar{Name: k, Value: v.Value, ValueFrom: v.ValueFrom})
	}
	mergedEnv = append(mergedEnv, kapi.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAME", Value: deployment.Name})
	mergedEnv = append(mergedEnv, kapi.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAMESPACE", Value: deployment.Namespace})

	// Inherit resources from the base container
	resources := kapi.ResourceRequirements{}
	if err := kapi.Scheme.Convert(&baseContainer.Resources, &resources, nil); err != nil {
		return nil, fmt.Errorf("couldn't clone ResourceRequirements: %v", err)
	}

	// Assigning to a variable since its address is required
	maxDeploymentDurationSeconds := deployapi.MaxDeploymentDurationSeconds - int64(time.Since(deployerPod.Status.StartTime.Time).Seconds())

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

	gracePeriod := int64(10)

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
			NodeSelector:                  deployment.Spec.Template.Spec.NodeSelector,
			RestartPolicy:                 restartPolicy,
			ImagePullSecrets:              imagePullSecrets,
			TerminationGracePeriodSeconds: &gracePeriod,
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

// NewPodWatch creates a pod watching function which is backed by a
// FIFO/reflector pair. This avoids managing watches directly.
// A stop channel to close the watch's reflector is also returned.
// It is the caller's responsibility to defer closing the stop channel to prevent leaking resources.
func NewPodWatch(client kclient.PodInterface, namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name)
	podLW := &cache.ListWatch{
		ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.List(options)
		},
		WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector
			return client.Watch(options)
		},
	}

	queue := cache.NewResyncableFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(podLW, &kapi.Pod{}, queue, 1*time.Minute).RunUntil(stopChannel)

	return func() *kapi.Pod {
		obj := cache.Pop(queue)
		return obj.(*kapi.Pod)
	}
}

// NewAcceptNewlyObservedReadyPods makes a new AcceptNewlyObservedReadyPods
// from a real client.
func NewAcceptNewlyObservedReadyPods(
	out io.Writer,
	kclient kclient.PodsNamespacer,
	timeout time.Duration,
	interval time.Duration,
	minReadySeconds int32,
) *AcceptNewlyObservedReadyPods {

	return &AcceptNewlyObservedReadyPods{
		out:             out,
		timeout:         timeout,
		interval:        interval,
		minReadySeconds: minReadySeconds,
		acceptedPods:    sets.NewString(),
		getDeploymentPodStore: func(deployment *kapi.ReplicationController) (cache.Store, chan struct{}) {
			selector := labels.Set(deployment.Spec.Selector).AsSelector()
			store := cache.NewStore(cache.MetaNamespaceKeyFunc)
			lw := &cache.ListWatch{
				ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
					options.LabelSelector = selector
					return kclient.Pods(deployment.Namespace).List(options)
				},
				WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
					options.LabelSelector = selector
					return kclient.Pods(deployment.Namespace).Watch(options)
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
	// minReadySeconds is the minimum number of seconds for which a newly created
	// pod should be ready without any of its container crashing, for it to be
	// considered available.
	minReadySeconds int32
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
			if kdeployutil.IsPodAvailable(pod, c.minReadySeconds, time.Now()) {
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
