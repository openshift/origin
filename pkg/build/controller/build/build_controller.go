package build

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	clientv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	kexternalclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kexternalcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/core/v1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kexternalcoreinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions/core/v1"
	v1lister "k8s.io/kubernetes/pkg/client/listers/core/v1"
	kcontroller "k8s.io/kubernetes/pkg/controller"

	builddefaults "github.com/openshift/origin/pkg/build/admission/defaults"
	buildoverrides "github.com/openshift/origin/pkg/build/admission/overrides"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/apis/build/validation"
	buildclient "github.com/openshift/origin/pkg/build/client"
	"github.com/openshift/origin/pkg/build/controller/common"
	"github.com/openshift/origin/pkg/build/controller/policy"
	"github.com/openshift/origin/pkg/build/controller/strategy"
	buildinformer "github.com/openshift/origin/pkg/build/generated/informers/internalversion/build/internalversion"
	buildlister "github.com/openshift/origin/pkg/build/generated/listers/build/internalversion"
	buildutil "github.com/openshift/origin/pkg/build/util"
	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinformers "github.com/openshift/origin/pkg/image/generated/informers/internalversion/image/internalversion"
	imagelister "github.com/openshift/origin/pkg/image/generated/listers/image/internalversion"
)

const (
	maxRetries = 15
)

// BuildController watches builds and synchronizes them with their
// corresponding build pods
type BuildController struct {
	buildPatcher      buildclient.BuildPatcher
	buildLister       buildlister.BuildLister
	buildConfigGetter buildlister.BuildConfigLister
	buildDeleter      buildclient.BuildDeleter
	podClient         kexternalcoreclient.PodsGetter
	kubeClient        kclientset.Interface

	queue workqueue.RateLimitingInterface

	buildStore       buildlister.BuildLister
	secretStore      v1lister.SecretLister
	podStore         v1lister.PodLister
	imageStreamStore imagelister.ImageStreamLister

	podInformer   cache.SharedIndexInformer
	buildInformer cache.SharedIndexInformer

	buildStoreSynced       func() bool
	podStoreSynced         func() bool
	secretStoreSynced      func() bool
	imageStreamStoreSynced func() bool

	runPolicies    []policy.RunPolicy
	createStrategy buildPodCreationStrategy
	buildDefaults  builddefaults.BuildDefaults
	buildOverrides buildoverrides.BuildOverrides

	recorder record.EventRecorder
}

// BuildControllerParams is the set of parameters needed to
// create a new BuildController
type BuildControllerParams struct {
	BuildInformer       buildinformer.BuildInformer
	BuildConfigInformer buildinformer.BuildConfigInformer
	ImageStreamInformer imageinformers.ImageStreamInformer
	PodInformer         kexternalcoreinformers.PodInformer
	SecretInformer      kexternalcoreinformers.SecretInformer
	KubeClientInternal  kclientset.Interface
	KubeClientExternal  kexternalclientset.Interface
	OpenshiftClient     osclient.Interface
	DockerBuildStrategy *strategy.DockerBuildStrategy
	SourceBuildStrategy *strategy.SourceBuildStrategy
	CustomBuildStrategy *strategy.CustomBuildStrategy
	BuildDefaults       builddefaults.BuildDefaults
	BuildOverrides      buildoverrides.BuildOverrides
}

// NewBuildController creates a new BuildController.
func NewBuildController(params *BuildControllerParams) *BuildController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(params.KubeClientExternal.Core().RESTClient()).Events("")})

	buildClient := buildclient.NewOSClientBuildClient(params.OpenshiftClient)
	buildLister := params.BuildInformer.Lister()
	buildConfigGetter := params.BuildConfigInformer.Lister()
	c := &BuildController{
		buildPatcher:      buildClient,
		buildLister:       buildLister,
		buildConfigGetter: buildConfigGetter,
		buildDeleter:      buildClient,
		secretStore:       params.SecretInformer.Lister(),
		podClient:         params.KubeClientExternal.Core(),
		kubeClient:        params.KubeClientInternal,
		podInformer:       params.PodInformer.Informer(),
		podStore:          params.PodInformer.Lister(),
		buildInformer:     params.BuildInformer.Informer(),
		buildStore:        params.BuildInformer.Lister(),
		imageStreamStore:  params.ImageStreamInformer.Lister(),
		createStrategy: &typeBasedFactoryStrategy{
			dockerBuildStrategy: params.DockerBuildStrategy,
			sourceBuildStrategy: params.SourceBuildStrategy,
			customBuildStrategy: params.CustomBuildStrategy,
		},
		buildDefaults:  params.BuildDefaults,
		buildOverrides: params.BuildOverrides,

		queue:       workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		recorder:    eventBroadcaster.NewRecorder(kapi.Scheme, clientv1.EventSource{Component: "build-controller"}),
		runPolicies: policy.GetAllRunPolicies(buildLister, buildClient),
	}

	c.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.podUpdated,
		DeleteFunc: c.podDeleted,
	})
	c.buildInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.buildAdded,
		UpdateFunc: c.buildUpdated,
	})

	c.buildStoreSynced = c.buildInformer.HasSynced
	c.podStoreSynced = c.podInformer.HasSynced
	c.secretStoreSynced = params.SecretInformer.Informer().HasSynced
	c.imageStreamStoreSynced = params.ImageStreamInformer.Informer().HasSynced

	return c
}

// Run begins watching and syncing.
func (c *BuildController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	// Wait for the controller stores to sync before starting any work in this controller.
	if !cache.WaitForCacheSync(stopCh, c.buildStoreSynced, c.podStoreSynced, c.secretStoreSynced, c.imageStreamStoreSynced) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	glog.Infof("Starting build controller")

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
	glog.Infof("Shutting down build controller")
}

func (bc *BuildController) worker() {
	for {
		if quit := bc.work(); quit {
			return
		}
	}
}

// work gets the next build from the queue and invokes handleBuild on it
func (bc *BuildController) work() bool {
	key, quit := bc.queue.Get()
	if quit {
		return true
	}

	defer bc.queue.Done(key)

	build, err := bc.getBuildByKey(key.(string))
	if err != nil {
		bc.handleError(err, key)
		return false
	}
	if build == nil {
		return false
	}

	err = bc.handleBuild(build)

	bc.handleError(err, key)
	return false
}

// handleBuild retrieves the build's corresponding pod and calls the appropriate
// handle function based on the build's current state. Each handler returns a buildUpdate
// object that includes any updates that need to be made on the build.
func (bc *BuildController) handleBuild(build *buildapi.Build) error {
	if shouldIgnore(build) {
		return nil
	}

	glog.V(4).Infof("Handling build %s", buildDesc(build))

	pod, podErr := bc.podStore.Pods(build.Namespace).Get(buildapi.GetBuildPodName(build))

	// Technically the only error that is returned from retrieving the pod is the
	// NotFound error so this check should not be needed, but leaving here in case
	// that changes in the future.
	if podErr != nil && !errors.IsNotFound(podErr) {
		return podErr
	}

	var update *buildUpdate
	var err, updateErr error

	switch {
	case shouldCancel(build):
		update, err = bc.cancelBuild(build)
	case build.Status.Phase == buildapi.BuildPhaseNew:
		update, err = bc.handleNewBuild(build, pod)
	case build.Status.Phase == buildapi.BuildPhasePending,
		build.Status.Phase == buildapi.BuildPhaseRunning:
		update, err = bc.handleActiveBuild(build, pod)
	case buildutil.IsBuildComplete(build):
		update, err = bc.handleCompletedBuild(build, pod)
	}
	if update != nil && !update.isEmpty() {
		updateErr = bc.updateBuild(build, update, pod)
	}
	if err != nil {
		return err
	}
	if updateErr != nil {
		return updateErr
	}
	return nil
}

// shouldIgnore returns true if a build should be ignored by the controller.
// These include pipeline builds as well as builds that are in a terminal state.
// However if the build is either complete or failed and its completion timestamp
// has not been set, then it returns false so that the build's completion timestamp
// gets updated.
func shouldIgnore(build *buildapi.Build) bool {
	// If pipeline build, do nothing.
	// These builds are processed/updated/etc by the jenkins sync plugin
	if build.Spec.Strategy.JenkinsPipelineStrategy != nil {
		glog.V(4).Infof("Ignoring build %s with jenkins pipeline strategy", buildDesc(build))
		return true
	}

	// If a build is in a terminal state, ignore it; unless it is in a succeeded or failed
	// state and its completion time is not set, then we should at least attempt to set its
	// completion time if possible.
	if buildutil.IsBuildComplete(build) {
		switch build.Status.Phase {
		case buildapi.BuildPhaseComplete,
			buildapi.BuildPhaseFailed:
			if build.Status.CompletionTimestamp == nil {
				return false
			}
		}
		glog.V(4).Infof("Ignoring build %s in completed state", buildDesc(build))
		return true
	}

	return false
}

// shouldCancel returns true if a build is active and its cancellation flag is set
func shouldCancel(build *buildapi.Build) bool {
	return !buildutil.IsBuildComplete(build) && build.Status.Cancelled
}

// cancelBuild deletes a build pod and returns an update to mark the build as cancelled
func (bc *BuildController) cancelBuild(build *buildapi.Build) (*buildUpdate, error) {
	glog.V(4).Infof("Cancelling build %s", buildDesc(build))

	podName := buildapi.GetBuildPodName(build)
	err := bc.podClient.Pods(build.Namespace).Delete(podName, &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("could not delete build pod %s/%s to cancel build %s: %v", build.Namespace, podName, buildDesc(build), err)
	}

	return transitionToPhase(buildapi.BuildPhaseCancelled, buildapi.StatusReasonCancelledBuild, buildapi.StatusMessageCancelledBuild), nil
}

// handleNewBuild will check whether policy allows running the new build and if so, creates a pod
// for the build and returns an update to move it to the Pending phase
func (bc *BuildController) handleNewBuild(build *buildapi.Build, pod *v1.Pod) (*buildUpdate, error) {
	// If a pod was found, and it was created after the build was created, it
	// means that the build is active and its status should be updated
	if pod != nil {
		//TODO: Use a better way to determine whether the pod corresponds to the build (maybe using the owner field)
		if !pod.CreationTimestamp.Before(build.CreationTimestamp) {
			return bc.handleActiveBuild(build, pod)
		}
		// If a pod was created before the current build, move the build to error
		return transitionToPhase(buildapi.BuildPhaseError, buildapi.StatusReasonBuildPodExists, buildapi.StatusMessageBuildPodExists), nil
	}

	runPolicy := policy.ForBuild(build, bc.runPolicies)
	if runPolicy == nil {
		return nil, fmt.Errorf("unable to determine build policy for %s", buildDesc(build))
	}

	// The runPolicy decides whether to execute this build or not.
	if run, err := runPolicy.IsRunnable(build); err != nil || !run {
		return nil, err
	}

	return bc.createBuildPod(build)
}

// createPodSpec creates a pod spec for the given build
func (bc *BuildController) createPodSpec(originalBuild *buildapi.Build, ref string) (*v1.Pod, error) {
	// TODO(rhcarvalho)
	// The S2I and Docker builders expect build.Spec.Output.To to contain a
	// resolved reference to a Docker image. Since build.Spec is immutable, we
	// change a copy (that is never persisted) and pass it to
	// createStrategy.createBuildPod. We should make the builders use
	// build.Status.OutputDockerImageReference, which will make copying the build
	// unnecessary.
	build, err := buildutil.BuildDeepCopy(originalBuild)
	if err != nil {
		return nil, fmt.Errorf("unable to copy build %s: %v", buildDesc(originalBuild), err)
	}

	build.Status.OutputDockerImageReference = ref
	if build.Spec.Output.To != nil && len(build.Spec.Output.To.Name) != 0 {
		build.Spec.Output.To = &kapi.ObjectReference{
			Kind: "DockerImage",
			Name: ref,
		}
	}

	// Invoke the strategy to create a build pod.
	podSpec, err := bc.createStrategy.CreateBuildPod(build)
	if err != nil {
		if strategy.IsFatal(err) {
			return nil, &strategy.FatalError{Reason: fmt.Sprintf("failed to create a build pod spec for build %s/%s: %v", build.Namespace, build.Name, err)}
		}
		return nil, fmt.Errorf("failed to create a build pod spec for build %s/%s: %v", build.Namespace, build.Name, err)
	}
	if err := bc.buildDefaults.ApplyDefaults(podSpec); err != nil {
		return nil, fmt.Errorf("failed to apply build defaults for build %s/%s: %v", build.Namespace, build.Name, err)
	}
	if err := bc.buildOverrides.ApplyOverrides(podSpec); err != nil {
		return nil, fmt.Errorf("failed to apply build overrides for build %s/%s: %v", build.Namespace, build.Name, err)
	}

	// Handle resolving ValueFrom references in build environment variables
	if err := common.ResolveValueFrom(podSpec, bc.kubeClient); err != nil {
		return nil, err
	}
	return podSpec, nil
}

// resolveOutputDockerImageReference returns a reference to a Docker image
// computed from the buid.Spec.Output.To reference.
func (bc *BuildController) resolveOutputDockerImageReference(build *buildapi.Build) (string, error) {
	outputTo := build.Spec.Output.To
	if outputTo == nil || outputTo.Name == "" {
		return "", nil
	}
	var ref string
	switch outputTo.Kind {
	case "DockerImage":
		ref = outputTo.Name
	case "ImageStream", "ImageStreamTag":
		// TODO(smarterclayton): security, ensure that the reference image stream is actually visible
		namespace := outputTo.Namespace
		if len(namespace) == 0 {
			namespace = build.Namespace
		}

		var tag string
		streamName := outputTo.Name
		if outputTo.Kind == "ImageStreamTag" {
			var ok bool
			streamName, tag, ok = imageapi.SplitImageStreamTag(streamName)
			if !ok {
				return "", fmt.Errorf("the referenced image stream tag is invalid: %s", outputTo.Name)
			}
			tag = ":" + tag
		}
		stream, err := bc.imageStreamStore.ImageStreams(namespace).Get(streamName)
		if err != nil {
			if errors.IsNotFound(err) {
				return "", fmt.Errorf("the referenced output image stream %s/%s does not exist", namespace, streamName)
			}
			return "", fmt.Errorf("the referenced output image stream %s/%s could not be found by build %s/%s: %v", namespace, streamName, build.Namespace, build.Name, err)
		}
		if len(stream.Status.DockerImageRepository) == 0 {
			e := fmt.Errorf("the image stream %s/%s cannot be used as the output for build %s/%s because the integrated Docker registry is not configured and no external registry was defined", namespace, outputTo.Name, build.Namespace, build.Name)
			bc.recorder.Eventf(build, kapi.EventTypeWarning, "invalidOutput", "Error starting build: %v", e)
			return "", e
		}
		ref = fmt.Sprintf("%s%s", stream.Status.DockerImageRepository, tag)
	}
	return ref, nil
}

// createBuildPod creates a new pod to run a build
func (bc *BuildController) createBuildPod(build *buildapi.Build) (*buildUpdate, error) {

	update := &buildUpdate{}

	// Set the output Docker image reference.
	ref, err := bc.resolveOutputDockerImageReference(build)
	if err != nil {
		// If we cannot resolve the output reference, the output image stream
		// may not yet exist. The build should remain in the new state and show the
		// reason that it is still in the new state.
		update.setReason(buildapi.StatusReasonInvalidOutputReference)
		update.setMessage(buildapi.StatusMessageInvalidOutputRef)
		return update, err
	}

	// Create the build pod spec
	buildPod, err := bc.createPodSpec(build, ref)
	if err != nil {
		switch err.(type) {
		case common.ErrEnvVarResolver:
			update = transitionToPhase(buildapi.BuildPhaseError, buildapi.StatusReasonUnresolvableEnvironmentVariable, fmt.Sprintf("%v, %v", buildapi.StatusMessageUnresolvableEnvironmentVariable, err.Error()))
		default:
			update.setReason(buildapi.StatusReasonCannotCreateBuildPodSpec)
			update.setMessage(buildapi.StatusMessageCannotCreateBuildPodSpec)

		}
		// If an error occurred when creating the pod spec, it likely means
		// that the build is something we don't understand. For example, it could
		// have a strategy that we don't recognize. It will remain in New state
		// and be updated with the reason that it is still in New

		// The error will be logged, but will not be returned to the caller
		// to be retried. The reason is that there's really no external factor
		// that could cause the pod creation to fail; therefore no reason
		// to immediately retry processing the build.
		//
		// A scenario where this would happen is that we've introduced a
		// new build strategy in the master, but the old version of the controller
		// is still running. We don't want the old controller to move the
		// build to the error phase and we don't want it to keep actively retrying.
		utilruntime.HandleError(err)
		return update, nil
	}

	glog.V(4).Infof("Pod %s/%s for build %s is about to be created", build.Namespace, buildPod.Name, buildDesc(build))
	if _, err := bc.podClient.Pods(build.Namespace).Create(buildPod); err != nil {
		if errors.IsAlreadyExists(err) {
			bc.recorder.Eventf(build, kapi.EventTypeWarning, "FailedCreate", "Pod already exists: %s/%s", buildPod.Namespace, buildPod.Name)
			glog.V(4).Infof("Build pod %s/%s for build %s already exists", build.Namespace, buildPod.Name, buildDesc(build))

			// If the existing pod was created before this build, switch to the Error state.
			existingPod, err := bc.podClient.Pods(build.Namespace).Get(buildPod.Name, metav1.GetOptions{})
			if err == nil && existingPod.CreationTimestamp.Before(build.CreationTimestamp) {
				update = transitionToPhase(buildapi.BuildPhaseError, buildapi.StatusReasonBuildPodExists, buildapi.StatusMessageBuildPodExists)
				return update, nil
			}
			return nil, nil
		}
		// Log an event if the pod is not created (most likely due to quota denial).
		bc.recorder.Eventf(build, kapi.EventTypeWarning, "FailedCreate", "Error creating: %v", err)
		update.setReason(buildapi.StatusReasonCannotCreateBuildPod)
		update.setMessage(buildapi.StatusMessageCannotCreateBuildPod)
		return update, fmt.Errorf("failed to create build pod: %v", err)
	}
	glog.V(4).Infof("Created pod %s/%s for build %s", build.Namespace, buildPod.Name, buildDesc(build))
	update = transitionToPhase(buildapi.BuildPhasePending, "", "")
	update.setPodNameAnnotation(buildPod.Name)
	update.setOutputRef(ref)
	return update, nil
}

// handleActiveBuild handles a build in either pending or running state
func (bc *BuildController) handleActiveBuild(build *buildapi.Build, pod *v1.Pod) (*buildUpdate, error) {

	if pod == nil {
		pod = bc.findMissingPod(build)
		if pod == nil {
			glog.V(4).Infof("Failed to find the build pod for build %s. Moving it to Error state", buildDesc(build))
			return transitionToPhase(buildapi.BuildPhaseError, buildapi.StatusReasonBuildPodDeleted, buildapi.StatusMessageBuildPodDeleted), nil
		}
	}

	var update *buildUpdate
	switch pod.Status.Phase {
	case v1.PodPending:
		if build.Status.Phase != buildapi.BuildPhasePending {
			update = transitionToPhase(buildapi.BuildPhasePending, "", "")
		}
		if secret := build.Spec.Output.PushSecret; secret != nil && build.Status.Reason != buildapi.StatusReasonMissingPushSecret {
			if _, err := bc.secretStore.Secrets(build.Namespace).Get(secret.Name); err != nil && errors.IsNotFound(err) {
				glog.V(4).Infof("Setting reason for pending build to %q due to missing secret for %s", build.Status.Reason, buildDesc(build))
				update = transitionToPhase(buildapi.BuildPhasePending, buildapi.StatusReasonMissingPushSecret, buildapi.StatusMessageMissingPushSecret)
			}
		}
	case v1.PodRunning:
		if build.Status.Phase != buildapi.BuildPhaseRunning {
			update = transitionToPhase(buildapi.BuildPhaseRunning, "", "")
			if pod.Status.StartTime != nil {
				update.setStartTime(*pod.Status.StartTime)
			}
		}
	case v1.PodSucceeded:
		if build.Status.Phase != buildapi.BuildPhaseComplete {
			update = transitionToPhase(buildapi.BuildPhaseComplete, "", "")
		}
		if len(pod.Status.ContainerStatuses) == 0 {
			// no containers in the pod means something went terribly wrong, so the build
			// should be set to an error state
			glog.V(2).Infof("Setting build %s to error state because its pod has no containers", buildDesc(build))
			update = transitionToPhase(buildapi.BuildPhaseError, buildapi.StatusReasonNoBuildContainerStatus, buildapi.StatusMessageNoBuildContainerStatus)
		} else {
			for _, info := range pod.Status.ContainerStatuses {
				if info.State.Terminated != nil && info.State.Terminated.ExitCode != 0 {
					glog.V(2).Infof("Setting build %s to error state because a container in its pod has non-zero exit code", buildDesc(build))
					update = transitionToPhase(buildapi.BuildPhaseError, buildapi.StatusReasonFailedContainer, buildapi.StatusMessageFailedContainer)
					break
				}
			}
		}
	case v1.PodFailed:
		if build.Status.Phase != buildapi.BuildPhaseFailed {
			// If a DeletionTimestamp has been set, it means that the pod will
			// soon be deleted. The build should be transitioned to the Error phase.
			if pod.DeletionTimestamp != nil {
				update = transitionToPhase(buildapi.BuildPhaseError, buildapi.StatusReasonBuildPodDeleted, buildapi.StatusMessageBuildPodDeleted)
			} else {
				update = transitionToPhase(buildapi.BuildPhaseFailed, buildapi.StatusReasonGenericBuildFailed, buildapi.StatusMessageGenericBuildFailed)
			}
		}
	}
	return update, nil
}

// handleCompletedBuild will only be called on builds that are already in a terminal phase however, their completion timestamp
// has not been set.
func (bc *BuildController) handleCompletedBuild(build *buildapi.Build, pod *v1.Pod) (*buildUpdate, error) {
	// Make sure that the completion timestamp has not already been set
	if build.Status.CompletionTimestamp != nil {
		return nil, nil
	}

	update := &buildUpdate{}
	var podStartTime *metav1.Time
	if pod != nil {
		podStartTime = pod.Status.StartTime
	}
	setBuildCompletionTimestampAndDuration(build, podStartTime, update)

	return update, nil
}

// updateBuild is the single place where any update to a build is done in the build controller.
// It will check that the update is valid, peform any necessary processing such as calling HandleBuildCompletion,
// and apply the buildUpdate object as a patch.
func (bc *BuildController) updateBuild(build *buildapi.Build, update *buildUpdate, pod *v1.Pod) error {

	stateTransition := false
	// Check whether we are transitioning to a different build phase
	if update.phase != nil && (*update.phase) != build.Status.Phase {
		stateTransition = true
	} else if build.Status.Phase == buildapi.BuildPhaseFailed && update.completionTime != nil {
		// Treat a failed->failed update as a state transition when the completionTime is getting
		// updated. This will cause an event to be emitted and completion processing to trigger.
		// We get into this state when the pod updates the phase through the build/details subresource.
		// The phase, reason, and message are set, but no event has been emitted about the failure,
		// and the policy has not been given a chance to start the next build if one is waiting to
		// start.
		update.setPhase(buildapi.BuildPhaseFailed)
		stateTransition = true
	}

	if stateTransition {
		// Make sure that the transition is valid
		if !isValidTransition(build.Status.Phase, *update.phase) {
			return fmt.Errorf("invalid phase transition %s -> %s", buildDesc(build), *update.phase)
		}

		// Log that we are updating build status
		reasonText := ""
		if update.reason != nil && *update.reason != "" {
			reasonText = fmt.Sprintf(" ( %s )", *update.reason)
		}

		// Update build completion timestamp if transitioning to a terminal phase
		if buildutil.IsTerminalPhase(*update.phase) {
			var podStartTime *metav1.Time
			if pod != nil {
				podStartTime = pod.Status.StartTime
			}
			setBuildCompletionTimestampAndDuration(build, podStartTime, update)
		}
		glog.V(4).Infof("Updating build %s -> %s%s", buildDesc(build), *update.phase, reasonText)
	}

	// Ensure that a pod name annotation has been set on the build if a pod is available
	if update.podNameAnnotation == nil && !common.HasBuildPodNameAnnotation(build) && pod != nil {
		update.setPodNameAnnotation(pod.Name)
	}

	patchedBuild, err := bc.patchBuild(build, update)
	if err != nil {
		return err
	}

	// Emit events and handle build completion if transitioned to a terminal phase
	if stateTransition {
		switch *update.phase {
		case buildapi.BuildPhaseRunning:
			bc.recorder.Eventf(patchedBuild, kapi.EventTypeNormal, buildapi.BuildStartedEventReason, fmt.Sprintf(buildapi.BuildStartedEventMessage, patchedBuild.Namespace, patchedBuild.Name))
		case buildapi.BuildPhaseCancelled:
			bc.recorder.Eventf(patchedBuild, kapi.EventTypeNormal, buildapi.BuildCancelledEventReason, fmt.Sprintf(buildapi.BuildCancelledEventMessage, patchedBuild.Namespace, patchedBuild.Name))
		case buildapi.BuildPhaseComplete:
			bc.recorder.Eventf(patchedBuild, kapi.EventTypeNormal, buildapi.BuildCompletedEventReason, fmt.Sprintf(buildapi.BuildCompletedEventMessage, patchedBuild.Namespace, patchedBuild.Name))
		case buildapi.BuildPhaseError,
			buildapi.BuildPhaseFailed:
			bc.recorder.Eventf(patchedBuild, kapi.EventTypeNormal, buildapi.BuildFailedEventReason, fmt.Sprintf(buildapi.BuildFailedEventMessage, patchedBuild.Namespace, patchedBuild.Name))
		}
		if buildutil.IsTerminalPhase(*update.phase) {
			common.HandleBuildCompletion(patchedBuild, bc.buildLister, bc.buildConfigGetter, bc.buildDeleter, bc.runPolicies)
		}
	}
	return nil
}

// patchBuild generates a patch for the given build and buildUpdate
// and applies that patch using the REST client
func (bc *BuildController) patchBuild(build *buildapi.Build, update *buildUpdate) (*buildapi.Build, error) {

	// Create a patch using the buildUpdate object
	updatedBuild, err := buildutil.BuildDeepCopy(build)
	if err != nil {
		return nil, fmt.Errorf("cannot create a deep copy of build %s: %v", buildDesc(build), err)
	}
	update.apply(updatedBuild)
	patch, err := validation.CreateBuildPatch(build, updatedBuild)
	if err != nil {
		return nil, fmt.Errorf("failed to create a build patch: %v", err)
	}

	glog.V(5).Infof("Patching build %s with %v", buildDesc(build), update)
	return bc.buildPatcher.Patch(build.Namespace, build.Name, patch)
}

// findMissingPod uses the REST client directly to determine if a pod exists or not.
// It is called when a corresponding pod for a build is not found in the cache.
func (bc *BuildController) findMissingPod(build *buildapi.Build) *v1.Pod {
	// Make one last attempt to fetch the pod using the REST client
	pod, err := bc.podClient.Pods(build.Namespace).Get(buildapi.GetBuildPodName(build), metav1.GetOptions{})
	if err == nil {
		glog.V(2).Infof("Found missing pod for build %s by using direct client.", buildDesc(build))
		return pod
	}
	return nil
}

// getBuildByKey looks up a build by key in the buildInformer cache
func (bc *BuildController) getBuildByKey(key string) (*buildapi.Build, error) {
	obj, exists, err := bc.buildInformer.GetIndexer().GetByKey(key)
	if err != nil {
		glog.V(2).Infof("Unable to retrieve build %q from store: %v", key, err)
		return nil, err
	}
	if !exists {
		glog.V(2).Infof("Build %q has been deleted", key)
		return nil, nil
	}

	return obj.(*buildapi.Build), nil
}

// podUpdated gets called by the pod informer event handler whenever a pod
// is updated or there is a relist of pods
func (bc *BuildController) podUpdated(old, cur interface{}) {
	// A periodic relist will send update events for all known pods.
	curPod := cur.(*v1.Pod)
	oldPod := old.(*v1.Pod)
	// The old and new ResourceVersion will be the same in a relist of pods.
	// Here we ignore pod relists because we already listen to build relists.
	if curPod.ResourceVersion == oldPod.ResourceVersion {
		return
	}
	if isBuildPod(curPod) {
		bc.enqueueBuildForPod(curPod)
	}
}

// podDeleted gets called by the pod informer event handler whenever a pod
// is deleted
func (bc *BuildController) podDeleted(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone: %+v", obj))
			return
		}
		pod, ok = tombstone.Obj.(*v1.Pod)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a pod: %+v", obj))
			return
		}
	}
	if isBuildPod(pod) {
		bc.enqueueBuildForPod(pod)
	}
}

// buildAdded is called by the build informer event handler whenever a build
// is created
func (bc *BuildController) buildAdded(obj interface{}) {
	build := obj.(*buildapi.Build)
	bc.enqueueBuild(build)
}

// buildUpdated is called by the build informer event handler whenever a build
// is updated or there is a relist of builds
func (bc *BuildController) buildUpdated(old, cur interface{}) {
	build := cur.(*buildapi.Build)
	bc.enqueueBuild(build)
}

// enqueueBuild adds the given build to the queue.
func (bc *BuildController) enqueueBuild(build *buildapi.Build) {
	key, err := kcontroller.KeyFunc(build)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for build %#v: %v", build, err))
		return
	}
	bc.queue.Add(key)
}

// enqueueBuildForPod adds the build corresponding to the given pod to the controller
// queue. If a build is not found for the pod, then an error is logged.
func (bc *BuildController) enqueueBuildForPod(pod *v1.Pod) {
	bc.queue.Add(fmt.Sprintf("%s/%s", pod.Namespace, buildutil.GetBuildName(pod)))
}

// handleError is called by the main work loop to check the return of calling handleBuild.
// If an error occurred, then the key is re-added to the queue unless it has been retried too many
// times.
func (bc *BuildController) handleError(err error, key interface{}) {
	if err == nil {
		bc.queue.Forget(key)
		return
	}

	if strategy.IsFatal(err) {
		glog.V(2).Infof("Will not retry fatal error for key %v: %v", key, err)
		bc.queue.Forget(key)
		return
	}

	if bc.queue.NumRequeues(key) < maxRetries {
		glog.V(4).Infof("Retrying key %v: %v", key, err)
		bc.queue.AddRateLimited(key)
		return
	}

	glog.V(2).Infof("Giving up retrying %v: %v", key, err)
	bc.queue.Forget(key)
}

// isBuildPod returns true if the given pod is a build pod
func isBuildPod(pod *v1.Pod) bool {
	return len(buildutil.GetBuildName(pod)) > 0
}

// buildDesc is a utility to format the namespace/name and phase of a build
// for errors and logging
func buildDesc(build *buildapi.Build) string {
	return fmt.Sprintf("%s/%s (%s)", build.Namespace, build.Name, build.Status.Phase)
}

// transitionToPhase returns a buildUpdate object to transition a build to a new
// phase with the given reason and message
func transitionToPhase(phase buildapi.BuildPhase, reason buildapi.StatusReason, message string) *buildUpdate {
	update := &buildUpdate{}
	update.setPhase(phase)
	update.setReason(reason)
	update.setMessage(message)
	return update
}

// isValidTransition returns true if the given phase transition is valid
func isValidTransition(from, to buildapi.BuildPhase) bool {
	if from == to {
		return true
	}

	switch {
	case buildutil.IsTerminalPhase(from):
		return false
	case from == buildapi.BuildPhasePending:
		switch to {
		case buildapi.BuildPhaseNew:
			return false
		}
	case from == buildapi.BuildPhaseRunning:
		switch to {
		case buildapi.BuildPhaseNew,
			buildapi.BuildPhasePending:
			return false
		}
	}

	return true
}

// setBuildCompletionTimestampAndDuration sets the build completion time and duration as well as the start time
// if not already set on the given buildUpdate object
func setBuildCompletionTimestampAndDuration(build *buildapi.Build, podStartTime *metav1.Time, update *buildUpdate) {
	now := metav1.Now()
	update.setCompletionTime(now)

	startTime := build.Status.StartTimestamp
	if startTime == nil {
		if podStartTime != nil {
			startTime = podStartTime
		} else {
			startTime = &now
		}
		update.setStartTime(*startTime)
	}
	update.setDuration(now.Rfc3339Copy().Time.Sub(startTime.Rfc3339Copy().Time))
}
