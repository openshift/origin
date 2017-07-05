package support

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"
	kapipod "k8s.io/kubernetes/pkg/api/pod"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	strategyutil "github.com/openshift/origin/pkg/deploy/strategy/util"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/util"
	namer "github.com/openshift/origin/pkg/util/namer"
)

// hookContainerName is the name used for the container that runs inside hook pods.
const hookContainerName = "lifecycle"

// HookExecutor knows how to execute a deployment lifecycle hook.
type HookExecutor interface {
	Execute(hook *deployapi.LifecycleHook, rc *kapi.ReplicationController, suffix, label string) error
}

// hookExecutor implements the HookExecutor interface.
var _ HookExecutor = &hookExecutor{}

// hookExecutor executes a deployment lifecycle hook.
type hookExecutor struct {
	// pods provides client to pods
	pods kcoreclient.PodsGetter
	// tags allows setting image stream tags
	tags client.ImageStreamTagsNamespacer
	// out is where hook pod logs should be written to.
	out io.Writer
	// decoder is used for encoding/decoding.
	decoder runtime.Decoder
	// recorder is used to emit events from hooks
	events kcoreclient.EventsGetter
	// getPodLogs knows how to get logs from a pod and is used for testing
	getPodLogs func(*kapi.Pod) (io.ReadCloser, error)
}

// NewHookExecutor makes a HookExecutor from a client.
func NewHookExecutor(pods kcoreclient.PodsGetter, tags client.ImageStreamTagsNamespacer, events kcoreclient.EventsGetter, out io.Writer, decoder runtime.Decoder) HookExecutor {
	executor := &hookExecutor{
		tags:    tags,
		pods:    pods,
		events:  events,
		out:     out,
		decoder: decoder,
	}
	executor.getPodLogs = func(pod *kapi.Pod) (io.ReadCloser, error) {
		opts := &kapi.PodLogOptions{
			Container:  hookContainerName,
			Follow:     true,
			Timestamps: false,
		}
		return executor.pods.Pods(pod.Namespace).GetLogs(pod.Name, opts).Stream()
	}
	return executor
}

// Execute executes hook in the context of deployment. The suffix is used to
// distinguish the kind of hook (e.g. pre, post).
func (e *hookExecutor) Execute(hook *deployapi.LifecycleHook, rc *kapi.ReplicationController, suffix, label string) error {
	var err error
	switch {
	case len(hook.TagImages) > 0:
		tagEventMessages := []string{}
		for _, t := range hook.TagImages {
			image, ok := findContainerImage(rc, t.ContainerName)
			if ok {
				tagEventMessages = append(tagEventMessages, fmt.Sprintf("image %q as %q", image, t.To.Name))
			}
		}
		strategyutil.RecordConfigEvent(e.events, rc, e.decoder, kapi.EventTypeNormal, "Started",
			fmt.Sprintf("Running %s-hook (TagImages) %s for rc %s/%s", label, strings.Join(tagEventMessages, ","), rc.Namespace, rc.Name))
		err = e.tagImages(hook, rc, suffix, label)
	case hook.ExecNewPod != nil:
		strategyutil.RecordConfigEvent(e.events, rc, e.decoder, kapi.EventTypeNormal, "Started",
			fmt.Sprintf("Running %s-hook (%q) for rc %s/%s", label, strings.Join(hook.ExecNewPod.Command, " "), rc.Namespace, rc.Name))
		err = e.executeExecNewPod(hook, rc, suffix, label)
	}

	if err == nil {
		strategyutil.RecordConfigEvent(e.events, rc, e.decoder, kapi.EventTypeNormal, "Completed",
			fmt.Sprintf("The %s-hook for rc %s/%s completed successfully", label, rc.Namespace, rc.Name))
		return nil
	}

	// Retry failures are treated the same as Abort.
	switch hook.FailurePolicy {
	case deployapi.LifecycleHookFailurePolicyAbort, deployapi.LifecycleHookFailurePolicyRetry:
		strategyutil.RecordConfigEvent(e.events, rc, e.decoder, kapi.EventTypeWarning, "Failed",
			fmt.Sprintf("The %s-hook failed: %v, aborting rollout of %s/%s", label, err, rc.Namespace, rc.Name))
		return fmt.Errorf("the %s hook failed: %v, aborting rollout of %s/%s", label, err, rc.Namespace, rc.Name)
	case deployapi.LifecycleHookFailurePolicyIgnore:
		strategyutil.RecordConfigEvent(e.events, rc, e.decoder, kapi.EventTypeWarning, "Failed",
			fmt.Sprintf("The %s-hook failed: %v (ignore), rollout of %s/%s will continue", label, err, rc.Namespace, rc.Name))
		return nil
	default:
		return err
	}
}

// findContainerImage returns the image with the given container name from a replication controller.
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

// tagImages tags images as part of the lifecycle of a rc. It uses an ImageStreamTag client
// which will provision an ImageStream if it doesn't already exist.
func (e *hookExecutor) tagImages(hook *deployapi.LifecycleHook, rc *kapi.ReplicationController, suffix, label string) error {
	var errs []error
	for _, action := range hook.TagImages {
		value, ok := findContainerImage(rc, action.ContainerName)
		if !ok {
			errs = append(errs, fmt.Errorf("unable to find image for container %q, container could not be found", action.ContainerName))
			continue
		}
		namespace := action.To.Namespace
		if len(namespace) == 0 {
			namespace = rc.Namespace
		}
		if _, err := e.tags.ImageStreamTags(namespace).Update(&imageapi.ImageStreamTag{
			ObjectMeta: metav1.ObjectMeta{
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
// the hook parameters and replication controller. The pod is then synchronously
// watched until the pod completes, and if the pod failed, an error is returned.
//
// The hook pod inherits the following from the container the hook refers to:
//
//   * Environment (hook keys take precedence)
//   * Working directory
//   * Resources
func (e *hookExecutor) executeExecNewPod(hook *deployapi.LifecycleHook, rc *kapi.ReplicationController, suffix, label string) error {
	config, err := deployutil.DecodeDeploymentConfig(rc, e.decoder)
	if err != nil {
		return err
	}

	deployerPod, err := e.pods.Pods(rc.Namespace).Get(deployutil.DeployerPodNameForDeployment(rc.Name), metav1.GetOptions{})
	if err != nil {
		return err
	}
	var startTime time.Time
	// if the deployer pod has not yet had its status updated, it means the execution of the pod is racing with the kubelet
	// status update. Until kubernetes/kubernetes#36813 is implemented, this check will remain racy. Set to Now() expecting
	// that the kubelet is unlikely to be very far behind.
	if deployerPod.Status.StartTime != nil {
		startTime = deployerPod.Status.StartTime.Time
	} else {
		startTime = time.Now()
	}

	// Build a pod spec from the hook config and replication controller.
	podSpec, err := makeHookPod(hook, rc, &config.Spec.Strategy, suffix, startTime)
	if err != nil {
		return err
	}

	// Track whether the pod has already run to completion and avoid showing logs
	// or the Success message twice.
	completed, created := false, false

	// Try to create the pod.
	pod, err := e.pods.Pods(rc.Namespace).Create(podSpec)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return fmt.Errorf("couldn't create lifecycle pod for %s: %v", rc.Name, err)
		}
		completed = true
		pod = podSpec
		pod.Namespace = rc.Namespace
	} else {
		created = true
		fmt.Fprintf(e.out, "--> %s: Running hook pod ...\n", label)
	}

	stopChannel := make(chan struct{})
	defer close(stopChannel)
	nextPod := newPodWatch(e.pods.Pods(pod.Namespace), pod.Namespace, pod.Name, pod.ResourceVersion, stopChannel)

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
func (e *hookExecutor) readPodLogs(pod *kapi.Pod, wg *sync.WaitGroup) {
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

// makeHookPod makes a pod spec from a hook and replication controller.
func makeHookPod(hook *deployapi.LifecycleHook, rc *kapi.ReplicationController, strategy *deployapi.DeploymentStrategy, suffix string, startTime time.Time) (*kapi.Pod, error) {
	exec := hook.ExecNewPod
	var baseContainer *kapi.Container
	for _, container := range rc.Spec.Template.Spec.Containers {
		if container.Name == exec.ContainerName {
			baseContainer = &container
			break
		}
	}
	if baseContainer == nil {
		return nil, fmt.Errorf("no container named '%s' found in rc template", exec.ContainerName)
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
	mergedEnv = append(mergedEnv, kapi.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAME", Value: rc.Name})
	mergedEnv = append(mergedEnv, kapi.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAMESPACE", Value: rc.Namespace})

	// Inherit resources from the base container
	resources := kapi.ResourceRequirements{}
	if err := kapi.Scheme.Convert(&baseContainer.Resources, &resources, nil); err != nil {
		return nil, fmt.Errorf("couldn't clone ResourceRequirements: %v", err)
	}

	// Assigning to a variable since its address is required
	defaultActiveDeadline := deployapi.MaxDeploymentDurationSeconds
	if strategy.ActiveDeadlineSeconds != nil {
		defaultActiveDeadline = *(strategy.ActiveDeadlineSeconds)
	}
	maxDeploymentDurationSeconds := defaultActiveDeadline - int64(time.Since(startTime).Seconds())

	// Let the kubelet manage retries if requested
	restartPolicy := kapi.RestartPolicyNever
	if hook.FailurePolicy == deployapi.LifecycleHookFailurePolicyRetry {
		restartPolicy = kapi.RestartPolicyOnFailure
	}

	// Transfer any requested volumes to the hook pod.
	volumes := []kapi.Volume{}
	volumeNames := sets.NewString()
	for _, volume := range rc.Spec.Template.Spec.Volumes {
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
	for _, pullSecret := range rc.Spec.Template.Spec.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, kapi.LocalObjectReference{Name: pullSecret.Name})
	}

	gracePeriod := int64(10)

	var podSecurityContextCopy *kapi.PodSecurityContext
	if ctx, err := kapi.Scheme.DeepCopy(rc.Spec.Template.Spec.SecurityContext); err != nil {
		return nil, fmt.Errorf("unable to copy pod securityContext: %v", err)
	} else {
		podSecurityContextCopy = ctx.(*kapi.PodSecurityContext)
	}

	var securityContextCopy *kapi.SecurityContext
	if ctx, err := kapi.Scheme.DeepCopy(baseContainer.SecurityContext); err != nil {
		return nil, fmt.Errorf("unable to copy securityContext: %v", err)
	} else {
		securityContextCopy = ctx.(*kapi.SecurityContext)
	}

	pod := &kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: namer.GetPodName(rc.Name, suffix),
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: rc.Name,
			},
			Labels: map[string]string{
				deployapi.DeploymentPodTypeLabel:        suffix,
				deployapi.DeployerPodForDeploymentLabel: rc.Name,
			},
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:            hookContainerName,
					Image:           baseContainer.Image,
					ImagePullPolicy: baseContainer.ImagePullPolicy,
					Command:         exec.Command,
					WorkingDir:      baseContainer.WorkingDir,
					Env:             mergedEnv,
					Resources:       resources,
					VolumeMounts:    volumeMounts,
					SecurityContext: securityContextCopy,
				},
			},
			SecurityContext:       podSecurityContextCopy,
			Volumes:               volumes,
			ActiveDeadlineSeconds: &maxDeploymentDurationSeconds,
			// Setting the node selector on the hook pod so that it is created
			// on the same set of nodes as the rc pods.
			NodeSelector:                  rc.Spec.Template.Spec.NodeSelector,
			RestartPolicy:                 restartPolicy,
			ImagePullSecrets:              imagePullSecrets,
			TerminationGracePeriodSeconds: &gracePeriod,
		},
	}

	util.MergeInto(pod.Labels, strategy.Labels, 0)
	util.MergeInto(pod.Annotations, strategy.Annotations, 0)

	return pod, nil
}

// canRetryReading returns whether the deployment strategy can retry reading logs
// from the given (hook) pod and the number of restarts that pod has.
func canRetryReading(pod *kapi.Pod, restarts int32) (bool, int32) {
	if len(pod.Status.ContainerStatuses) == 0 {
		return false, int32(0)
	}
	restartCount := pod.Status.ContainerStatuses[0].RestartCount
	return pod.Spec.RestartPolicy == kapi.RestartPolicyOnFailure && restartCount > restarts, restartCount
}

// newPodWatch creates a pod watching function which is backed by a
// FIFO/reflector pair. This avoids managing watches directly.
// A stop channel to close the watch's reflector is also returned.
// It is the caller's responsibility to defer closing the stop channel to prevent leaking resources.
func newPodWatch(client kcoreclient.PodInterface, namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name)
	podLW := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector.String()
			return client.List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector.String()
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

// NewAcceptAvailablePods makes a new acceptAvailablePods from a real client.
func NewAcceptAvailablePods(
	out io.Writer,
	kclient kcoreclient.PodsGetter,
	timeout time.Duration,
	interval time.Duration,
	minReadySeconds int32,
) *acceptAvailablePods {

	return &acceptAvailablePods{
		out:             out,
		timeout:         timeout,
		interval:        interval,
		minReadySeconds: minReadySeconds,
		acceptedPods:    sets.NewString(),
		getRcPodStore: func(rc *kapi.ReplicationController) (cache.Store, chan struct{}) {
			selector := labels.Set(rc.Spec.Selector).AsSelector()
			store := cache.NewStore(cache.MetaNamespaceKeyFunc)
			lw := &cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					options.LabelSelector = selector.String()
					return kclient.Pods(rc.Namespace).List(options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					options.LabelSelector = selector.String()
					return kclient.Pods(rc.Namespace).Watch(options)
				},
			}
			stop := make(chan struct{})
			cache.NewReflector(lw, &kapi.Pod{}, store, 10*time.Second).RunUntil(stop)
			return store, stop
		},
	}
}

// acceptAvailablePods will accept a replication controller if all the pods
// for the replication controller become available.
//
// acceptAvailablePods keeps track of the pods it has accepted for a
// replication controller so that the acceptor can be reused across multiple
// batches of updates to a single controller. For example, if during the first
// acceptance call the replication controller has 3 pods, the acceptor will
// validate those 3 pods. If the same acceptor instance is used again for the
// same replication controller which now has 6 pods, only the latest 3 pods
// will be considered for acceptance. The status of the original 3 pods becomes
// irrelevant.
//
// Note that this struct is stateful and intended for use with a single
// replication controller and should be discarded and recreated between
// rollouts.
type acceptAvailablePods struct {
	out io.Writer
	// getRcPodStore should return a Store containing all the pods for the
	// replication controller, and a channel to stop whatever process is
	// feeding the store.
	getRcPodStore func(*kapi.ReplicationController) (cache.Store, chan struct{})
	// timeout is how long to wait for pod readiness.
	timeout time.Duration
	// interval is how often to check for pod readiness
	interval time.Duration
	// minReadySeconds is the minimum number of seconds for which a newly created
	// pod should be ready without any of its container crashing, for it to be
	// considered available.
	minReadySeconds int32
	// acceptedPods keeps track of pods which have been previously accepted for
	// a replication controller.
	acceptedPods sets.String
}

// Accept all pods for a replication controller once they are available.
func (c *acceptAvailablePods) Accept(rc *kapi.ReplicationController) error {
	// Make a pod store to poll and ensure it gets cleaned up.
	podStore, stopStore := c.getRcPodStore(rc)
	defer close(stopStore)

	// Start checking for pod updates.
	if c.acceptedPods.Len() > 0 {
		fmt.Fprintf(c.out, "--> Waiting up to %s for pods in rc %s to become ready (%d pods previously accepted)\n", c.timeout, rc.Name, c.acceptedPods.Len())
	} else {
		fmt.Fprintf(c.out, "--> Waiting up to %s for pods in rc %s to become ready\n", c.timeout, rc.Name)
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
			if kapipod.IsPodAvailable(pod, c.minReadySeconds, metav1.NewTime(time.Now())) {
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
		glog.V(4).Infof("Still waiting for %d pods to become ready for rc %s", unready.Len(), rc.Name)
		return false, nil
	})

	// Handle acceptance failure.
	if err != nil {
		if err == wait.ErrWaitTimeout {
			return fmt.Errorf("pods for rc %q took longer than %.f seconds to become ready", rc.Name, c.timeout.Seconds())
		}
		return fmt.Errorf("pod readiness check failed for rc %q: %v", rc.Name, err)
	}
	return nil
}
