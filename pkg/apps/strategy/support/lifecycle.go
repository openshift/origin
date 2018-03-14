package support

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	"github.com/openshift/origin/pkg/api/apihelpers"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	strategyutil "github.com/openshift/origin/pkg/apps/strategy/util"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
	"github.com/openshift/origin/pkg/util"
)

// hookContainerName is the name used for the container that runs inside hook pods.
const hookContainerName = "lifecycle"

// HookExecutor knows how to execute a deployment lifecycle hook.
type HookExecutor interface {
	Execute(hook *appsapi.LifecycleHook, rc *kapi.ReplicationController, suffix, label string) error
}

// hookExecutor implements the HookExecutor interface.
var _ HookExecutor = &hookExecutor{}

// hookExecutor executes a deployment lifecycle hook.
type hookExecutor struct {
	// pods provides client to pods
	pods kcoreclient.PodsGetter
	// tags allows setting image stream tags
	tags imageclient.ImageStreamTagsGetter
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
func NewHookExecutor(pods kcoreclient.PodsGetter, tags imageclient.ImageStreamTagsGetter, events kcoreclient.EventsGetter, out io.Writer, decoder runtime.Decoder) HookExecutor {
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
func (e *hookExecutor) Execute(hook *appsapi.LifecycleHook, rc *kapi.ReplicationController, suffix, label string) error {
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
	case appsapi.LifecycleHookFailurePolicyAbort, appsapi.LifecycleHookFailurePolicyRetry:
		strategyutil.RecordConfigEvent(e.events, rc, e.decoder, kapi.EventTypeWarning, "Failed",
			fmt.Sprintf("The %s-hook failed: %v, aborting rollout of %s/%s", label, err, rc.Namespace, rc.Name))
		return fmt.Errorf("the %s hook failed: %v, aborting rollout of %s/%s", label, err, rc.Namespace, rc.Name)
	case appsapi.LifecycleHookFailurePolicyIgnore:
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
func (e *hookExecutor) tagImages(hook *appsapi.LifecycleHook, rc *kapi.ReplicationController, suffix, label string) error {
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
func (e *hookExecutor) executeExecNewPod(hook *appsapi.LifecycleHook, rc *kapi.ReplicationController, suffix, label string) error {
	config, err := appsutil.DecodeDeploymentConfig(rc, e.decoder)
	if err != nil {
		return err
	}

	deployerPod, err := e.pods.Pods(rc.Namespace).Get(appsutil.DeployerPodNameForDeployment(rc.Name), metav1.GetOptions{})
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
	nextPod := newPodWatch(e.pods.Pods(pod.Namespace), pod.Name, stopChannel)

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
func makeHookPod(hook *appsapi.LifecycleHook, rc *kapi.ReplicationController, strategy *appsapi.DeploymentStrategy, suffix string, startTime time.Time) (*kapi.Pod, error) {
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
	if err := legacyscheme.Scheme.Convert(&baseContainer.Resources, &resources, nil); err != nil {
		return nil, fmt.Errorf("couldn't clone ResourceRequirements: %v", err)
	}

	// Assigning to a variable since its address is required
	defaultActiveDeadline := appsapi.MaxDeploymentDurationSeconds
	if strategy.ActiveDeadlineSeconds != nil {
		defaultActiveDeadline = *(strategy.ActiveDeadlineSeconds)
	}
	maxDeploymentDurationSeconds := defaultActiveDeadline - int64(time.Since(startTime).Seconds())

	// Let the kubelet manage retries if requested
	restartPolicy := kapi.RestartPolicyNever
	if hook.FailurePolicy == appsapi.LifecycleHookFailurePolicyRetry {
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
				SubPath:   mount.SubPath,
			})
		}
	}

	// Transfer image pull secrets from the pod spec.
	imagePullSecrets := []kapi.LocalObjectReference{}
	for _, pullSecret := range rc.Spec.Template.Spec.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, kapi.LocalObjectReference{Name: pullSecret.Name})
	}

	gracePeriod := int64(10)
	podSecurityContextCopy := rc.Spec.Template.Spec.SecurityContext.DeepCopy()
	securityContextCopy := baseContainer.SecurityContext.DeepCopy()

	pod := &kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: apihelpers.GetPodName(rc.Name, suffix),
			Annotations: map[string]string{
				appsapi.DeploymentAnnotation: rc.Name,
			},
			Labels: map[string]string{
				appsapi.DeploymentPodTypeLabel:        suffix,
				appsapi.DeployerPodForDeploymentLabel: rc.Name,
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
func newPodWatch(client kcoreclient.PodInterface, name string, stopChannel chan struct{}) func() *kapi.Pod {
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
	go cache.NewReflector(podLW, &kapi.Pod{}, queue, 1*time.Minute).Run(stopChannel)

	return func() *kapi.Pod {
		obj := cache.Pop(queue)
		return obj.(*kapi.Pod)
	}
}

// NewAcceptAvailablePods makes a new acceptAvailablePods from a real client.
func NewAcceptAvailablePods(
	out io.Writer,
	kclient kcoreclient.ReplicationControllersGetter,
	timeout time.Duration,
) *acceptAvailablePods {
	return &acceptAvailablePods{
		out:     out,
		kclient: kclient,
		timeout: timeout,
	}
}

// acceptAvailablePods will accept a replication controller if all the pods
// for the replication controller become available.
type acceptAvailablePods struct {
	out     io.Writer
	kclient kcoreclient.ReplicationControllersGetter
	// timeout is how long to wait for pods to become available from ready state.
	timeout time.Duration
}

// Accept all pods for a replication controller once they are available.
func (c *acceptAvailablePods) Accept(rc *kapi.ReplicationController) error {
	allReplicasAvailable := func(r *kapi.ReplicationController) bool {
		return r.Status.AvailableReplicas == r.Spec.Replicas
	}

	if allReplicasAvailable(rc) {
		return nil
	}

	watcher, err := c.kclient.ReplicationControllers(rc.Namespace).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: rc.Name, ResourceVersion: rc.ResourceVersion}))
	if err != nil {
		return fmt.Errorf("acceptAvailablePods failed to watch ReplicationController %s/%s: %v", rc.Namespace, rc.Name, err)
	}

	_, err = watch.Until(c.timeout, watcher, func(event watch.Event) (bool, error) {
		if t := event.Type; t != watch.Modified {
			return false, fmt.Errorf("acceptAvailablePods failed watching for ReplicationController %s/%s: received event %v", rc.Namespace, rc.Name, t)
		}
		newRc := event.Object.(*kapi.ReplicationController)
		return allReplicasAvailable(newRc), nil
	})
	// Handle acceptance failure.
	if err != nil {
		if err == wait.ErrWaitTimeout {
			return fmt.Errorf("pods for rc '%s/%s' took longer than %.f seconds to become available", rc.Namespace, rc.Name, c.timeout.Seconds())
		}
		return err
	}
	return nil
}
