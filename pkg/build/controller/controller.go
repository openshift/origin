package controller

import (
	"fmt"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	errors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/record"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"

	builddefaults "github.com/openshift/origin/pkg/build/admission/defaults"
	buildoverrides "github.com/openshift/origin/pkg/build/admission/overrides"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	"github.com/openshift/origin/pkg/build/controller/common"
	"github.com/openshift/origin/pkg/build/controller/policy"
	strategy "github.com/openshift/origin/pkg/build/controller/strategy"
	buildutil "github.com/openshift/origin/pkg/build/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// BuildController watches build resources and manages their state
type BuildController struct {
	BuildUpdater      buildclient.BuildUpdater
	BuildLister       buildclient.BuildLister
	PodManager        podManager
	BuildStrategy     BuildStrategy
	ImageStreamClient imageStreamClient
	Recorder          record.EventRecorder
	RunPolicies       []policy.RunPolicy
	BuildDefaults     builddefaults.BuildDefaults
	BuildOverrides    buildoverrides.BuildOverrides
}

// BuildStrategy knows how to create a pod spec for a pod which can execute a build.
type BuildStrategy interface {
	CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error)
}

type podManager interface {
	CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	DeletePod(namespace string, pod *kapi.Pod) error
	GetPod(namespace, name string) (*kapi.Pod, error)
}

type imageStreamClient interface {
	GetImageStream(namespace, name string) (*imageapi.ImageStream, error)
}

// CancelBuild updates a build status to Cancelled, after its associated pod is deleted.
func (bc *BuildController) CancelBuild(build *buildapi.Build) error {
	if !isBuildCancellable(build) {
		glog.V(4).Infof("Build %s/%s can be cancelled only if it has pending/running status, not %s.", build.Namespace, build.Name, build.Status.Phase)
		return nil
	}

	glog.V(4).Infof("Cancelling build %s/%s.", build.Namespace, build.Name)

	pod, err := bc.PodManager.GetPod(build.Namespace, buildapi.GetBuildPodName(build))
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("Failed to get pod for build %s/%s: %v", build.Namespace, build.Name, err)
		}
	} else {
		err := bc.PodManager.DeletePod(build.Namespace, pod)
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("Couldn't delete build pod %s/%s: %v", build.Namespace, pod.Name, err)
		}
	}

	build.Status.Phase = buildapi.BuildPhaseCancelled
	common.SetBuildCompletionTimeAndDuration(build)
	// set the status details for the cancelled build before updating the build
	// object.
	build.Status.Reason = buildapi.StatusReasonCancelledBuild
	build.Status.Message = buildapi.StatusMessageCancelledBuild
	if err := bc.BuildUpdater.Update(build.Namespace, build); err != nil {
		return fmt.Errorf("Failed to update build %s/%s: %v", build.Namespace, build.Name, err)
	}

	glog.V(4).Infof("Build %s/%s was successfully cancelled.", build.Namespace, build.Name)

	common.HandleBuildCompletion(build, bc.RunPolicies)

	return nil
}

// HandleBuild deletes pods for cancelled builds and takes new builds and puts
// them in the pending state after creating a corresponding pod
func (bc *BuildController) HandleBuild(build *buildapi.Build) error {
	// these builds are processed/updated/etc by the jenkins sync plugin
	if build.Spec.Strategy.JenkinsPipelineStrategy != nil {
		glog.V(4).Infof("Ignoring build with jenkins pipeline strategy")
		return nil
	}
	glog.V(4).Infof("Handling build %s/%s (%s)", build.Namespace, build.Name, build.Status.Phase)

	// A cancelling event was triggered for the build, delete its pod and update build status.
	if build.Status.Cancelled && build.Status.Phase != buildapi.BuildPhaseCancelled {
		glog.V(5).Infof("Marking build %s/%s as cancelled", build.Namespace, build.Name)
		if err := bc.CancelBuild(build); err != nil {
			build.Status.Reason = buildapi.StatusReasonCancelBuildFailed
			build.Status.Message = buildapi.StatusMessageCancelBuildFailed
			if err = bc.BuildUpdater.Update(build.Namespace, build); err != nil {
				utilruntime.HandleError(fmt.Errorf("Failed to update build %s/%s: %v", build.Namespace, build.Name, err))
			}
			return fmt.Errorf("Failed to cancel build %s/%s: %v, will retry", build.Namespace, build.Name, err)
		}
	}

	// Handle only new builds from this point
	if build.Status.Phase != buildapi.BuildPhaseNew {
		return nil
	}

	runPolicy := policy.ForBuild(build, bc.RunPolicies)
	if runPolicy == nil {
		return fmt.Errorf("unable to determine build scheduler for %s/%s", build.Namespace, build.Name)
	}

	// The runPolicy decides whether to execute this build or not.
	if run, err := runPolicy.IsRunnable(build); err != nil || !run {
		return err
	}

	if err := bc.nextBuildPhase(build); err != nil {
		return err
	}

	if err := bc.BuildUpdater.Update(build.Namespace, build); err != nil {
		// This is not a retryable error because the build has been created.  The worst case
		// outcome of not updating the buildconfig is that we might rerun a build for the
		// same "new" imageid change in the future, which is better than guaranteeing we
		// run the build 2+ times by retrying it here.
		glog.V(2).Infof("Failed to record changes to build %s/%s: %v", build.Namespace, build.Name, err)
	}
	return nil
}

// nextBuildPhase updates build with any appropriate changes, or returns an error if
// the change cannot occur. When returning nil, be sure to set build.Status and optionally
// build.Message.
func (bc *BuildController) nextBuildPhase(build *buildapi.Build) error {
	// If a cancelling event was triggered for the build, update build status.
	if build.Status.Cancelled {
		glog.V(4).Infof("Cancelling build %s/%s.", build.Namespace, build.Name)
		build.Status.Phase = buildapi.BuildPhaseCancelled
		return nil
	}

	// Set the output Docker image reference.
	ref, err := bc.resolveOutputDockerImageReference(build)
	if err != nil {
		build.Status.Reason = buildapi.StatusReasonInvalidOutputReference
		build.Status.Message = buildapi.StatusMessageInvalidOutputRef
		return err
	}
	build.Status.OutputDockerImageReference = ref

	// Make a copy to avoid mutating the build from this point on.
	copy, err := kapi.Scheme.Copy(build)
	if err != nil {
		return fmt.Errorf("unable to copy build: %v", err)
	}
	buildCopy := copy.(*buildapi.Build)

	// TODO(rhcarvalho)
	// The S2I and Docker builders expect build.Spec.Output.To to contain a
	// resolved reference to a Docker image. Since build.Spec is immutable, we
	// change a copy (that is never persisted) and pass it to
	// bc.BuildStrategy.CreateBuildPod. We should make the builders use
	// build.Status.OutputDockerImageReference, what will make copying the build
	// unnecessary.
	if build.Spec.Output.To != nil && len(build.Spec.Output.To.Name) != 0 {
		buildCopy.Spec.Output.To = &kapi.ObjectReference{
			Kind: "DockerImage",
			Name: ref,
		}
	}

	// Invoke the strategy to get a build pod.
	podSpec, err := bc.BuildStrategy.CreateBuildPod(buildCopy)
	if err != nil {
		build.Status.Reason = buildapi.StatusReasonCannotCreateBuildPodSpec
		build.Status.Message = buildapi.StatusMessageCannotCreateBuildPodSpec
		if strategy.IsFatal(err) {
			return &strategy.FatalError{Reason: fmt.Sprintf("failed to create a build pod spec for build %s/%s: %v", build.Namespace, build.Name, err)}
		}
		return fmt.Errorf("failed to create a build pod spec for build %s/%s: %v", build.Namespace, build.Name, err)
	}
	if err := bc.BuildDefaults.ApplyDefaults(podSpec); err != nil {
		return fmt.Errorf("failed to apply build defaults for build %s/%s: %v", build.Namespace, build.Name, err)
	}
	if err := bc.BuildOverrides.ApplyOverrides(podSpec); err != nil {
		return fmt.Errorf("failed to apply build overrides for build %s/%s: %v", build.Namespace, build.Name, err)
	}

	glog.V(4).Infof("Pod %s for build %s/%s is about to be created", podSpec.Name, build.Namespace, build.Name)

	if _, err := bc.PodManager.CreatePod(build.Namespace, podSpec); err != nil {
		if errors.IsAlreadyExists(err) {
			bc.Recorder.Eventf(build, kapi.EventTypeWarning, "FailedCreate", "Pod already exists: %s/%s", podSpec.Namespace, podSpec.Name)
			glog.V(4).Infof("Build pod already existed: %#v", podSpec)

			// If the existing pod was created before this build, switch to Error state.
			existingPod, err := bc.PodManager.GetPod(podSpec.Namespace, podSpec.Name)
			if err == nil && existingPod.CreationTimestamp.Before(build.CreationTimestamp) {
				build.Status.Phase = buildapi.BuildPhaseError
				build.Status.Reason = buildapi.StatusReasonBuildPodExists
				build.Status.Message = buildapi.StatusMessageBuildPodExists
			}
			return nil
		}
		// Log an event if the pod is not created (most likely due to quota denial).
		bc.Recorder.Eventf(build, kapi.EventTypeWarning, "FailedCreate", "Error creating: %v", err)
		build.Status.Reason = buildapi.StatusReasonCannotCreateBuildPod
		build.Status.Message = buildapi.StatusMessageCannotCreateBuildPod
		return fmt.Errorf("failed to create build pod: %v", err)
	}
	common.SetBuildPodNameAnnotation(build, podSpec.Name)
	glog.V(4).Infof("Created pod for build: %#v", podSpec)

	// Set the build phase, which will be persisted.
	build.Status.Phase = buildapi.BuildPhasePending
	build.Status.Reason = ""
	build.Status.Message = ""
	return nil
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
		stream, err := bc.ImageStreamClient.GetImageStream(namespace, streamName)
		if err != nil {
			if errors.IsNotFound(err) {
				return "", fmt.Errorf("the referenced output image stream %s/%s does not exist", namespace, streamName)
			}
			return "", fmt.Errorf("the referenced output image stream %s/%s could not be found by build %s/%s: %v", namespace, streamName, build.Namespace, build.Name, err)
		}
		if len(stream.Status.DockerImageRepository) == 0 {
			e := fmt.Errorf("the image stream %s/%s cannot be used as the output for build %s/%s because the integrated Docker registry is not configured and no external registry was defined", namespace, outputTo.Name, build.Namespace, build.Name)
			bc.Recorder.Eventf(build, kapi.EventTypeWarning, "invalidOutput", "Error starting build: %v", e)
			return "", e
		}
		ref = fmt.Sprintf("%s%s", stream.Status.DockerImageRepository, tag)
	}
	return ref, nil
}

// isBuildCancellable checks for build status and returns true if the condition is checked.
func isBuildCancellable(build *buildapi.Build) bool {
	return build.Status.Phase == buildapi.BuildPhaseNew || build.Status.Phase == buildapi.BuildPhasePending || build.Status.Phase == buildapi.BuildPhaseRunning
}

// BuildDeleteController watches for builds being deleted and cleans up associated pods
type BuildDeleteController struct {
	PodManager podManager
}

// HandleBuildDeletion deletes a build pod if the corresponding build has been deleted
func (bc *BuildDeleteController) HandleBuildDeletion(build *buildapi.Build) error {
	glog.V(4).Infof("Handling deletion of build %s", build.Name)
	if build.Spec.Strategy.JenkinsPipelineStrategy != nil {
		glog.V(4).Infof("Ignoring build with jenkins pipeline strategy")
		return nil
	}
	podName := buildapi.GetBuildPodName(build)
	pod, err := bc.PodManager.GetPod(build.Namespace, podName)
	if err != nil && !errors.IsNotFound(err) {
		glog.V(2).Infof("Failed to find pod with name %s for build %s in namespace %s due to error: %v", podName, build.Name, build.Namespace, err)
		return err
	}
	if pod == nil {
		glog.V(2).Infof("Did not find pod with name %s for build %s in namespace %s", podName, build.Name, build.Namespace)
		return nil
	}
	if buildName := buildapi.GetBuildName(pod); buildName != build.Name {
		glog.V(2).Infof("Not deleting pod %s/%s because the build label %s does not match the build name %s", pod.Namespace, podName, buildName, build.Name)
		return nil
	}
	err = bc.PodManager.DeletePod(build.Namespace, pod)
	if err != nil && !errors.IsNotFound(err) {
		glog.V(2).Infof("Failed to delete pod %s/%s for build %s due to error: %v", build.Namespace, podName, build.Name, err)
		return err
	}
	return nil
}

// buildKey returns a build object that can be used to lookup a build
// in the cache store, given a pod for the build
func buildKey(pod *kapi.Pod) *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name:      buildutil.GetBuildName(pod),
			Namespace: pod.Namespace,
		},
	}
}
