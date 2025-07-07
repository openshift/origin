package images

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/reference"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/klog/v2"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsv1 "github.com/openshift/api/apps/v1"
	imageapiv1 "github.com/openshift/api/image/v1"
	imageclienttyped "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	"github.com/openshift/library-go/pkg/build/naming"
	"github.com/openshift/origin/test/extended/scheme"

	"github.com/openshift/library-go/pkg/apps/appsserialization"
	"github.com/openshift/library-go/pkg/apps/appsutil"
)

const (
	// hookContainerName is the name used for the container that runs inside hook pods.
	hookContainerName = "lifecycle"
	// deploymentPodTypeLabel is a label with which contains a type of deployment pod.
	deploymentPodTypeLabel = "openshift.io/deployer-pod.type"
	// deploymentAnnotation is an annotation on a deployer Pod. The annotation value is the name
	// of the deployment (a ReplicationController) on which the deployer Pod acts.
	deploymentAnnotation = "openshift.io/deployment.name"
)

// HookExecutor knows how to execute a deployment lifecycle hook.
type HookExecutor interface {
	Execute(hook *appsv1.LifecycleHook, rc *corev1.ReplicationController, suffix, label string) error
}

// hookExecutor implements the HookExecutor interface.
var _ HookExecutor = &hookExecutor{}

// hookExecutor executes a deployment lifecycle hook.
type hookExecutor struct {
	// pods provides client to pods
	pods corev1client.PodsGetter
	// tags allows setting image stream tags
	tags imageclienttyped.ImageStreamTagsGetter
	// out is where hook pod logs should be written to.
	out io.Writer
	// recorder is used to emit events from hooks
	events corev1client.EventsGetter
	// getPodLogs knows how to get logs from a pod and is used for testing
	getPodLogs func(*corev1.Pod) (io.ReadCloser, error)
}

// NewHookExecutor makes a HookExecutor from a client.
func NewHookExecutor(kubeClient kubernetes.Interface, imageClient imageclienttyped.ImageStreamTagsGetter, out io.Writer) HookExecutor {
	executor := &hookExecutor{
		tags:   imageClient,
		pods:   kubeClient.CoreV1(),
		events: kubeClient.CoreV1(),
		out:    out,
	}
	executor.getPodLogs = func(pod *corev1.Pod) (io.ReadCloser, error) {
		opts := &corev1.PodLogOptions{
			Container:  hookContainerName,
			Follow:     true,
			Timestamps: false,
		}
		return executor.pods.Pods(pod.Namespace).GetLogs(pod.Name, opts).Stream(context.Background())
	}
	return executor
}

// Execute executes hook in the context of deployment. The suffix is used to
// distinguish the kind of hook (e.g. pre, post).
func (e *hookExecutor) Execute(hook *appsv1.LifecycleHook, rc *corev1.ReplicationController, suffix, label string) error {
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
		RecordConfigEvent(e.events, rc, kapi.EventTypeNormal, "Started",
			fmt.Sprintf("Running %s-hook (TagImages) %s for rc %s/%s", label, strings.Join(tagEventMessages, ","), rc.Namespace, rc.Name))
		err = e.tagImages(hook, rc, suffix, label)
	case hook.ExecNewPod != nil:
		RecordConfigEvent(e.events, rc, kapi.EventTypeNormal, "Started",
			fmt.Sprintf("Running %s-hook (%q) for rc %s/%s", label, strings.Join(hook.ExecNewPod.Command, " "), rc.Namespace, rc.Name))
		err = e.executeExecNewPod(hook, rc, suffix, label)
	}

	if err == nil {
		RecordConfigEvent(e.events, rc, kapi.EventTypeNormal, "Completed",
			fmt.Sprintf("The %s-hook for rc %s/%s completed successfully", label, rc.Namespace, rc.Name))
		return nil
	}

	// Retry failures are treated the same as Abort.
	switch hook.FailurePolicy {
	case appsv1.LifecycleHookFailurePolicyAbort, appsv1.LifecycleHookFailurePolicyRetry:
		RecordConfigEvent(e.events, rc, kapi.EventTypeWarning, "Failed",
			fmt.Sprintf("The %s-hook failed: %v, aborting rollout of %s/%s", label, err, rc.Namespace, rc.Name))
		return fmt.Errorf("the %s hook failed: %v, aborting rollout of %s/%s", label, err, rc.Namespace, rc.Name)
	case appsv1.LifecycleHookFailurePolicyIgnore:
		RecordConfigEvent(e.events, rc, kapi.EventTypeWarning, "Failed",
			fmt.Sprintf("The %s-hook failed: %v (ignore), rollout of %s/%s will continue", label, err, rc.Namespace, rc.Name))
		return nil
	default:
		return err
	}
}

// findContainerImage returns the image with the given container name from a replication controller.
func findContainerImage(rc *corev1.ReplicationController, containerName string) (string, bool) {
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
func (e *hookExecutor) tagImages(hook *appsv1.LifecycleHook, rc *corev1.ReplicationController, suffix, label string) error {
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
		if _, err := e.tags.ImageStreamTags(namespace).Update(context.Background(), &imageapiv1.ImageStreamTag{
			ObjectMeta: metav1.ObjectMeta{
				Name:      action.To.Name,
				Namespace: namespace,
			},
			Tag: &imageapiv1.TagReference{
				From: &corev1.ObjectReference{
					Kind: "DockerImage",
					Name: value,
				},
			},
		}, metav1.UpdateOptions{}); err != nil {
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
//   - Environment (hook keys take precedence)
//   - Working directory
//   - Resources
func (e *hookExecutor) executeExecNewPod(hook *appsv1.LifecycleHook, rc *corev1.ReplicationController, suffix, label string) error {
	config, err := appsserialization.DecodeDeploymentConfig(rc)
	if err != nil {
		return err
	}

	deployerPod, err := e.pods.Pods(rc.Namespace).Get(context.Background(), appsutil.DeployerPodNameForDeployment(rc.Name), metav1.GetOptions{})
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
	podSpec, err := createHookPodManifest(hook, rc, &config.Spec.Strategy, suffix, startTime)
	if err != nil {
		return err
	}

	// Track whether the pod has already run to completion and avoid showing logs
	// or the Success message twice.
	completed, created := false, false

	// Try to create the pod.
	pod, err := e.pods.Pods(rc.Namespace).Create(context.Background(), podSpec, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("couldn't create lifecycle pod for %s: %v", rc.Name, err)
		}
		completed = true
		pod = podSpec
		pod.Namespace = rc.Namespace
	} else {
		created = true
		fmt.Fprintf(e.out, "--> %s: Running hook pod ...\n", label)
	}

	var updatedPod *corev1.Pod
	restarts := int32(0)
	alreadyRead := false
	wg := &sync.WaitGroup{}
	wg.Add(1)

	listWatcher := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", pod.Name).String()
			return e.pods.Pods(pod.Namespace).List(context.Background(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", pod.Name).String()
			return e.pods.Pods(pod.Namespace).Watch(context.Background(), options)
		},
	}
	// make sure that the pod exists and wasn't deleted early
	preconditionFunc := func(store cache.Store) (bool, error) {
		_, exists, err := store.Get(&metav1.ObjectMeta{Namespace: pod.Namespace, Name: pod.Name})
		if err != nil {
			return true, err
		}
		if !exists {
			// We need to make sure we see the object in the cache before we start waiting for events
			// or we would be waiting for the timeout if such object didn't exist.
			return true, apierrors.NewNotFound(corev1.Resource("pods"), pod.Name)
		}

		return false, nil
	}
	// Wait for the hook pod to reach a terminal phase. Start reading logs as
	// soon as the pod enters a usable phase.
	_, err = watchtools.UntilWithSync(
		context.TODO(),
		listWatcher,
		&corev1.Pod{},
		preconditionFunc,
		func(event watch.Event) (bool, error) {
			switch event.Type {
			case watch.Error:
				return false, apierrors.FromObject(event.Object)
			case watch.Added, watch.Modified:
				updatedPod = event.Object.(*corev1.Pod)
			case watch.Deleted:
				err := fmt.Errorf("%s: pod/%s[%s] unexpectedly deleted", label, pod.Name, pod.Namespace)
				fmt.Fprintf(e.out, "%v\n", err)
				return false, err

			}

			switch updatedPod.Status.Phase {
			case corev1.PodRunning:
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

			case corev1.PodSucceeded, corev1.PodFailed:
				if completed {
					if updatedPod.Status.Phase == corev1.PodSucceeded {
						fmt.Fprintf(e.out, "--> %s: Hook pod already succeeded\n", label)
					}
					wg.Done()
					return true, nil
				}
				if !created {
					fmt.Fprintf(e.out, "--> %s: Hook pod is already running ...\n", label)
				}
				if !alreadyRead {
					go e.readPodLogs(pod, wg)
				}
				return true, nil
			default:
				completed = false
			}

			return false, nil
		},
	)
	if err != nil {
		return err
	}

	// The pod is finished, wait for all logs to be consumed before returning.
	wg.Wait()
	if updatedPod.Status.Phase == corev1.PodFailed {
		fmt.Fprintf(e.out, "--> %s: Failed\n", label)
		return fmt.Errorf("%s", updatedPod.Status.Message)
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
func (e *hookExecutor) readPodLogs(pod *corev1.Pod, wg *sync.WaitGroup) {
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

func createHookPodManifest(hook *appsv1.LifecycleHook, rc *corev1.ReplicationController, strategy *appsv1.DeploymentStrategy,
	hookType string,
	startTime time.Time) (*corev1.Pod, error) {

	exec := hook.ExecNewPod

	var baseContainer *corev1.Container

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
	envMap := map[string]corev1.EnvVar{}
	mergedEnv := []corev1.EnvVar{}
	for _, env := range baseContainer.Env {
		envMap[env.Name] = env
	}
	for _, env := range exec.Env {
		envMap[env.Name] = env
	}
	for k, v := range envMap {
		mergedEnv = append(mergedEnv, corev1.EnvVar{Name: k, Value: v.Value, ValueFrom: v.ValueFrom})
	}
	mergedEnv = append(mergedEnv, corev1.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAME", Value: rc.Name})
	mergedEnv = append(mergedEnv, corev1.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAMESPACE", Value: rc.Namespace})

	// Assigning to a variable since its address is required
	defaultActiveDeadline := appsutil.MaxDeploymentDurationSeconds
	if strategy.ActiveDeadlineSeconds != nil {
		defaultActiveDeadline = *(strategy.ActiveDeadlineSeconds)
	}
	maxDeploymentDurationSeconds := defaultActiveDeadline - int64(time.Since(startTime).Seconds())

	// Let the kubelet manage retries if requested
	restartPolicy := corev1.RestartPolicyNever
	if hook.FailurePolicy == appsv1.LifecycleHookFailurePolicyRetry {
		restartPolicy = corev1.RestartPolicyOnFailure
	}

	// Transfer any requested volumes to the hook pod.
	volumes := []corev1.Volume{}
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
	volumeMounts := []corev1.VolumeMount{}
	for _, mount := range baseContainer.VolumeMounts {
		if volumeNames.Has(mount.Name) {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      mount.Name,
				ReadOnly:  mount.ReadOnly,
				MountPath: mount.MountPath,
				SubPath:   mount.SubPath,
			})
		}
	}

	// Transfer image pull secrets from the pod spec.
	imagePullSecrets := []corev1.LocalObjectReference{}
	for _, pullSecret := range rc.Spec.Template.Spec.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: pullSecret.Name})
	}

	gracePeriod := int64(10)
	podSecurityContextCopy := rc.Spec.Template.Spec.SecurityContext.DeepCopy()
	securityContextCopy := baseContainer.SecurityContext.DeepCopy()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.GetPodName(rc.Name, hookType),
			Namespace: rc.Namespace,
			Annotations: map[string]string{
				deploymentAnnotation: rc.Name,
			},
			Labels: map[string]string{
				appsv1.DeployerPodForDeploymentLabel: rc.Name,
				deploymentPodTypeLabel:               hookType,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            hookContainerName,
					Image:           baseContainer.Image,
					ImagePullPolicy: baseContainer.ImagePullPolicy,
					Command:         exec.Command,
					WorkingDir:      baseContainer.WorkingDir,
					Env:             mergedEnv,
					Resources:       baseContainer.Resources,
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

	// add in DC specified labels and annotations
	for k, v := range strategy.Labels {
		if _, ok := pod.Labels[k]; ok {
			continue
		}
		pod.Labels[k] = v
	}
	for k, v := range strategy.Annotations {
		if _, ok := pod.Annotations[k]; ok {
			continue
		}
		pod.Annotations[k] = v
	}

	return pod, nil
}

// canRetryReading returns whether the deployment strategy can retry reading logs
// from the given (hook) pod and the number of restarts that pod has.
func canRetryReading(pod *corev1.Pod, restarts int32) (bool, int32) {
	if len(pod.Status.ContainerStatuses) == 0 {
		return false, int32(0)
	}
	restartCount := pod.Status.ContainerStatuses[0].RestartCount
	return pod.Spec.RestartPolicy == corev1.RestartPolicyOnFailure && restartCount > restarts, restartCount
}

// deployment.
func RecordConfigEvent(client corev1client.EventsGetter, deployment *corev1.ReplicationController, eventType, reason,
	msg string) {
	t := metav1.Time{Time: time.Now()}
	var obj runtime.Object = deployment
	if config, err := appsserialization.DecodeDeploymentConfig(deployment); err == nil {
		obj = config
	} else {
		klog.Errorf("Unable to decode deployment config from %s/%s: %v", deployment.Namespace, deployment.Name, err)
	}
	ref, err := reference.GetReference(scheme.Scheme, obj)
	if err != nil {
		klog.Errorf("Unable to get reference for %#v: %v", obj, err)
		return
	}
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.%x", ref.Name, t.UnixNano()),
			Namespace: ref.Namespace,
		},
		InvolvedObject: *ref,
		Reason:         reason,
		Message:        msg,
		Source: corev1.EventSource{
			Component: appsutil.DeployerPodNameFor(deployment),
		},
		FirstTimestamp: t,
		LastTimestamp:  t,
		Count:          1,
		Type:           eventType,
	}
	if _, err := client.Events(ref.Namespace).Create(context.Background(), event, metav1.CreateOptions{}); err != nil {
		klog.Errorf("Could not create event '%#v': %v", event, err)
	}
}

// RecordConfigWarnings records all warning events from the replication controller to the
// associated deployment config.
func RecordConfigWarnings(client corev1client.EventsGetter, rc *corev1.ReplicationController, out io.Writer) {
	if rc == nil {
		return
	}
	events, err := client.Events(rc.Namespace).Search(scheme.Scheme, rc)
	if err != nil {
		fmt.Fprintf(out, "--> Error listing events for replication controller %s: %v\n", rc.Name, err)
		return
	}
	// TODO: Do we need to sort the events?
	for _, e := range events.Items {
		if e.Type == corev1.EventTypeWarning {
			fmt.Fprintf(out, "-->  %s: %s %s\n", e.Reason, rc.Name, e.Message)
			RecordConfigEvent(client, rc, e.Type, e.Reason, e.Message)
		}
	}
}
