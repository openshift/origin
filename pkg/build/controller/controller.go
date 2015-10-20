package controller

import (
	"fmt"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	errors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/record"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildutil "github.com/openshift/origin/pkg/build/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// BuildController watches build resources and manages their state
type BuildController struct {
	BuildUpdater      buildclient.BuildUpdater
	PodManager        podManager
	BuildStrategy     BuildStrategy
	ImageStreamClient imageStreamClient
	Recorder          record.EventRecorder
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

	pod, err := bc.PodManager.GetPod(build.Namespace, buildutil.GetBuildPodName(build))
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
	now := unversioned.Now()
	build.Status.CompletionTimestamp = &now
	if err := bc.BuildUpdater.Update(build.Namespace, build); err != nil {
		return fmt.Errorf("Failed to update build %s/%s: %v", build.Namespace, build.Name, err)
	}

	glog.V(4).Infof("Build %s/%s was successfully cancelled.", build.Namespace, build.Name)
	return nil
}

// HandleBuild deletes pods for canceled builds and takes new builds and puts
// them in the pending state after creating a corresponding pod
func (bc *BuildController) HandleBuild(build *buildapi.Build) error {
	glog.V(4).Infof("Handling build %s/%s", build.Namespace, build.Name)

	// A cancelling event was triggered for the build, delete its pod and update build status.
	if build.Status.Cancelled && build.Status.Phase != buildapi.BuildPhaseCancelled {
		if err := bc.CancelBuild(build); err != nil {
			return fmt.Errorf("Failed to cancel build %s/%s: %v, will retry", build.Namespace, build.Name, err)
		}
	}

	// Handle new builds
	if build.Status.Phase != buildapi.BuildPhaseNew {
		return nil
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
		return err
	}
	build.Status.OutputDockerImageReference = ref

	// Set the build phase, which will be persisted if no error occurs.
	build.Status.Phase = buildapi.BuildPhasePending

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
		return fmt.Errorf("failed to create a build pod spec with strategy %q: %v", build.Spec.Strategy.Type, err)
	}
	glog.V(4).Infof("Pod %s for build %s/%s is about to be created", podSpec.Name, build.Namespace, build.Name)

	if _, err := bc.PodManager.CreatePod(build.Namespace, podSpec); err != nil {
		if errors.IsAlreadyExists(err) {
			glog.V(4).Infof("Build pod already existed: %#v", podSpec)
			return nil
		}
		// Log an event if the pod is not created (most likely due to quota denial).
		bc.Recorder.Eventf(build, "failedCreate", "Error creating: %v", err)
		return fmt.Errorf("failed to create build pod: %v", err)
	}

	glog.V(4).Infof("Created pod for build: %#v", podSpec)
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
			bc.Recorder.Eventf(build, "invalidOutput", "Error starting build: %v", e)
			return "", e
		}
		ref = fmt.Sprintf("%s%s", stream.Status.DockerImageRepository, tag)
	}
	return ref, nil
}

// BuildPodController watches pods running builds and manages the build state
type BuildPodController struct {
	BuildStore   cache.Store
	BuildUpdater buildclient.BuildUpdater
	PodManager   podManager
}

// HandlePod updates the state of the build based on the pod state
func (bc *BuildPodController) HandlePod(pod *kapi.Pod) error {
	obj, exists, err := bc.BuildStore.Get(buildKey(pod))
	if err != nil {
		glog.V(4).Infof("Error getting build for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		return err
	}
	if !exists || obj == nil {
		glog.V(5).Infof("No build found for pod %s/%s", pod.Namespace, pod.Name)
		return nil
	}

	build := obj.(*buildapi.Build)

	nextStatus := build.Status.Phase
	switch pod.Status.Phase {
	case kapi.PodRunning:
		// The pod's still running
		nextStatus = buildapi.BuildPhaseRunning
	case kapi.PodSucceeded:
		// Check the exit codes of all the containers in the pod
		nextStatus = buildapi.BuildPhaseComplete
		if len(pod.Status.ContainerStatuses) == 0 {
			// no containers in the pod means something went badly wrong, so the build
			// should be failed.
			glog.V(2).Infof("Failing build %s/%s because the pod has no containers", build.Namespace, build.Name)
			nextStatus = buildapi.BuildPhaseFailed
		} else {
			for _, info := range pod.Status.ContainerStatuses {
				if info.State.Terminated != nil && info.State.Terminated.ExitCode != 0 {
					nextStatus = buildapi.BuildPhaseFailed
					break
				}
			}
		}
	case kapi.PodFailed:
		nextStatus = buildapi.BuildPhaseFailed
	}

	if build.Status.Phase != nextStatus {
		glog.V(4).Infof("Updating build %s/%s status %s -> %s", build.Namespace, build.Name, build.Status.Phase, nextStatus)
		build.Status.Phase = nextStatus
		if buildutil.IsBuildComplete(build) {
			now := unversioned.Now()
			build.Status.CompletionTimestamp = &now
		}
		if build.Status.Phase == buildapi.BuildPhaseRunning {
			now := unversioned.Now()
			build.Status.StartTimestamp = &now
		}
		if err := bc.BuildUpdater.Update(build.Namespace, build); err != nil {
			return fmt.Errorf("failed to update build %s/%s: %v", build.Namespace, build.Name, err)
		}
		glog.V(4).Infof("Build %s/%s status was updated %s -> %s", build.Namespace, build.Name, build.Status.Phase, nextStatus)
	}
	return nil
}

// isBuildCancellable checks for build status and returns true if the condition is checked.
func isBuildCancellable(build *buildapi.Build) bool {
	return build.Status.Phase == buildapi.BuildPhaseNew || build.Status.Phase == buildapi.BuildPhasePending || build.Status.Phase == buildapi.BuildPhaseRunning
}

// BuildPodDeleteController watches pods running builds and updates the build if the pod is deleted
type BuildPodDeleteController struct {
	BuildStore   cache.Store
	BuildUpdater buildclient.BuildUpdater
}

// HandleBuildPodDeletion sets the status of a build to error if the build pod has been deleted
func (bc *BuildPodDeleteController) HandleBuildPodDeletion(pod *kapi.Pod) error {
	glog.V(4).Infof("Handling deletion of build pod %s/%s", pod.Namespace, pod.Name)
	obj, exists, err := bc.BuildStore.Get(buildKey(pod))
	if err != nil {
		glog.V(4).Infof("Error getting build for pod %s/%s", pod.Namespace, pod.Name)
		return err
	}
	if !exists || obj == nil {
		glog.V(5).Infof("No build found for deleted pod %s/%s", pod.Namespace, pod.Name)
		return nil
	}
	build := obj.(*buildapi.Build)

	// If build was cancelled, we'll leave HandleBuild to update the build
	if build.Status.Cancelled {
		glog.V(4).Infof("Cancelation for build was already triggered, ignoring")
		return nil
	}

	if buildutil.IsBuildComplete(build) {
		glog.V(4).Infof("Pod was deleted but build %s/%s is already completed, so no need to update it.", build.Namespace, build.Name)
		return nil
	}

	nextStatus := buildapi.BuildPhaseError
	if build.Status.Phase != nextStatus {
		glog.V(4).Infof("Updating build %s/%s status %s -> %s", build.Namespace, build.Name, build.Status.Phase, nextStatus)
		build.Status.Phase = nextStatus
		build.Status.Message = "The pod for this build was deleted before the build completed."
		now := unversioned.Now()
		build.Status.CompletionTimestamp = &now
		if err := bc.BuildUpdater.Update(build.Namespace, build); err != nil {
			return fmt.Errorf("Failed to update build %s/%s: %v", build.Namespace, build.Name, err)
		}
	}
	return nil
}

// BuildDeleteController watches for builds being deleted and cleans up associated pods
type BuildDeleteController struct {
	PodManager podManager
}

// HandleBuildDeletion deletes a build pod if the corresponding build has been deleted
func (bc *BuildDeleteController) HandleBuildDeletion(build *buildapi.Build) error {
	glog.V(4).Infof("Handling deletion of build %s", build.Name)
	podName := buildutil.GetBuildPodName(build)
	pod, err := bc.PodManager.GetPod(build.Namespace, podName)
	if err != nil && !errors.IsNotFound(err) {
		glog.V(2).Infof("Failed to find pod with name %s for build %s in namespace %s due to error: %v", podName, build.Name, build.Namespace, err)
		return err
	}
	if pod == nil {
		glog.V(2).Infof("Did not find pod with name %s for build %s in namespace %s", podName, build.Name, build.Namespace)
		return nil
	}
	if buildName := pod.Labels[buildapi.BuildLabel]; buildName != build.Name {
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
