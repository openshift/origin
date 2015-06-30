package controller

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	errors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

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

// HandleBuild takes new builds and puts them in the pending state after
// creating a corresponding pod
func (bc *BuildController) HandleBuild(build *buildapi.Build) error {
	glog.V(4).Infof("Handling Build %s/%s", build.Namespace, build.Name)

	// We only deal with new builds here
	if build.Status != buildapi.BuildStatusNew {
		return nil
	}

	if err := bc.nextBuildStatus(build); err != nil {
		return fmt.Errorf("Build failed with error %s/%s: %v", build.Namespace, build.Name, err)
	}

	if err := bc.BuildUpdater.Update(build.Namespace, build); err != nil {
		// This is not a retryable error because the build has been created.  The worst case
		// outcome of not updating the buildconfig is that we might rerun a build for the
		// same "new" imageid change in the future, which is better than guaranteeing we
		// run the build 2+ times by retrying it here.
		glog.V(2).Infof("Failed to record changes to Build %s/%s: %v", build.Namespace, build.Name, err)
	}
	return nil
}

// nextBuildStatus updates build with any appropriate changes, or returns an error if
// the change cannot occur. When returning nil, be sure to set build.Status and optionally
// build.Message.
func (bc *BuildController) nextBuildStatus(build *buildapi.Build) error {
	// If a cancelling event was triggered for the build, update build status.
	if build.Cancelled {
		glog.V(4).Infof("Cancelling Build %s/%s.", build.Namespace, build.Name)
		build.Status = buildapi.BuildStatusCancelled
		return nil
	}

	// lookup the destination from the referenced image repository
	spec := build.Parameters.Output.DockerImageReference
	if ref := build.Parameters.Output.To; ref != nil {
		// TODO: security, ensure that the reference image stream is actually visible
		namespace := ref.Namespace
		if len(namespace) == 0 {
			namespace = build.Namespace
		}

		repo, err := bc.ImageStreamClient.GetImageStream(namespace, ref.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("the referenced output ImageStream %s/%s does not exist", namespace, ref.Name)
			}
			return fmt.Errorf("the referenced output ImageStream %s/%s could not be found by Build %s/%s: %v", namespace, ref.Name, build.Namespace, build.Name, err)
		}
		if len(repo.Status.DockerImageRepository) == 0 {
			e := fmt.Errorf("the ImageStream %s/%s cannot be used as the output for Build %s/%s because the integrated Docker registry is not configured, or the user forgot to set a valid external registry", namespace, ref.Name, build.Namespace, build.Name)
			bc.Recorder.Eventf(build, "invalidOutput", "Error starting build: %v", e)
			return e
		}
		spec = repo.Status.DockerImageRepository
	}

	// set the expected build parameters, which will be saved if no error occurs
	build.Status = buildapi.BuildStatusPending

	// Make a copy to avoid mutating the build from this point on
	copy, err := kapi.Scheme.Copy(build)
	if err != nil {
		return fmt.Errorf("unable to copy Build: %v", err)
	}
	buildCopy := copy.(*buildapi.Build)

	// override DockerImageReference in the strategy for the copy we send to the build pod
	buildCopy.Parameters.Output.DockerImageReference = spec
	buildCopy.Parameters.Output.To = nil

	// invoke the strategy to get a build pod
	podSpec, err := bc.BuildStrategy.CreateBuildPod(buildCopy)
	if err != nil {
		return fmt.Errorf("the strategy failed to create a build pod for Build %s/%s: %v", build.Namespace, build.Name, err)
	}
	glog.V(4).Infof("Pod %s for Build %s/%s is about to be created", podSpec.Name, build.Namespace, build.Name)

	if _, err := bc.PodManager.CreatePod(build.Namespace, podSpec); err != nil {
		if errors.IsAlreadyExists(err) {
			glog.V(4).Infof("Build pod already existed: %#v", podSpec)
			return nil
		}
		// log an event if the pod is not created (most likely due to quota denial)
		bc.Recorder.Eventf(build, "failedCreate", "Error creating: %v", err)
		return fmt.Errorf("failed to create pod for Build %s/%s: %v", build.Namespace, build.Name, err)
	}

	glog.V(4).Infof("Created pod for Build: %#v", podSpec)
	return nil
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
		glog.V(4).Infof("Error getting Build for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		return err
	}
	if !exists || obj == nil {
		glog.V(5).Infof("No Build found for pod %s/%s", pod.Namespace, pod.Name)
		return nil
	}

	build := obj.(*buildapi.Build)

	// A cancelling event was triggered for the build, delete its pod and update build status.
	if build.Cancelled {
		glog.V(4).Infof("Cancelling Build %s/%s.", build.Namespace, build.Name)

		if err := bc.CancelBuild(build, pod); err != nil {
			return fmt.Errorf("failed to cancel Build %s/%s: %v, will retry", build.Namespace, build.Name, err)
		}
		return nil
	}

	nextStatus := build.Status

	switch pod.Status.Phase {
	case kapi.PodRunning:
		// The pod's still running
		nextStatus = buildapi.BuildStatusRunning
	case kapi.PodSucceeded, kapi.PodFailed:
		// Check the exit codes of all the containers in the pod
		nextStatus = buildapi.BuildStatusComplete
		for _, info := range pod.Status.ContainerStatuses {
			if info.State.Terminated != nil && info.State.Terminated.ExitCode != 0 {
				nextStatus = buildapi.BuildStatusFailed
				break
			}
		}
	}

	if build.Status != nextStatus {
		glog.V(4).Infof("Updating Build %s/%s status %s -> %s", build.Namespace, build.Name, build.Status, nextStatus)
		build.Status = nextStatus
		if buildutil.IsBuildComplete(build) {
			now := util.Now()
			build.CompletionTimestamp = &now
		}
		if build.Status == buildapi.BuildStatusRunning {
			now := util.Now()
			build.StartTimestamp = &now
		}
		if err := bc.BuildUpdater.Update(build.Namespace, build); err != nil {
			return fmt.Errorf("failed to update Build %s/%s: %v", build.Namespace, build.Name, err)
		}
		glog.V(4).Infof("Build %s/%s status was updated %s -> %s", build.Namespace, build.Name, build.Status, nextStatus)
	}
	return nil
}

// CancelBuild updates a build status to Cancelled, after its associated pod is deleted.
func (bc *BuildPodController) CancelBuild(build *buildapi.Build, pod *kapi.Pod) error {
	if !isBuildCancellable(build) {
		glog.V(4).Infof("Build %s/%s can be cancelled only if it has pending/running status, not %s.", build.Namespace, build.Name, build.Status)
		return nil
	}

	err := bc.PodManager.DeletePod(build.Namespace, pod)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	glog.V(4).Infof("Build %s/%s is about to be cancelled", build.Namespace, build.Name)
	build.Status = buildapi.BuildStatusCancelled
	now := util.Now()
	build.CompletionTimestamp = &now
	if err := bc.BuildUpdater.Update(build.Namespace, build); err != nil {
		return err
	}

	glog.V(4).Infof("Build %s/%s was successfully cancelled.", build.Namespace, build.Name)
	return nil
}

// isBuildCancellable checks for build status and returns true if the condition is checked.
func isBuildCancellable(build *buildapi.Build) bool {
	return build.Status == buildapi.BuildStatusNew || build.Status == buildapi.BuildStatusPending || build.Status == buildapi.BuildStatusRunning
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
		glog.V(5).Infof("No Build found for deleted pod %s/%s", pod.Namespace, pod.Name)
		return nil
	}
	build := obj.(*buildapi.Build)

	if buildutil.IsBuildComplete(build) {
		glog.V(4).Infof("Pod was deleted but Build %s/%s is already completed, so no need to update it.", build.Namespace, build.Name)
		return nil
	}

	nextStatus := buildapi.BuildStatusError
	if build.Status != nextStatus {
		glog.V(4).Infof("Updating build %s/%s status %s -> %s", build.Namespace, build.Name, build.Status, nextStatus)
		build.Status = nextStatus
		build.Message = "The Pod for this Build was deleted before the Build completed."
		now := util.Now()
		build.CompletionTimestamp = &now
		if err := bc.BuildUpdater.Update(build.Namespace, build); err != nil {
			return fmt.Errorf("Failed to update Build %s/%s: %v", build.Namespace, build.Name, err)
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
		glog.V(2).Infof("Failed to find pod with name %s for Build %s in namespace %s due to error: %v", podName, build.Name, build.Namespace, err)
		return err
	}
	if pod == nil {
		glog.V(2).Infof("Did not find pod with name %s for Build %s in namespace %s", podName, build.Name, build.Namespace)
		return nil
	}
	if pod.Labels[buildapi.BuildLabel] != build.Name {
		glog.V(2).Infof("Not deleting pod %s/%s because the build label %s does not match the build name %s", pod.Namespace, podName, pod.Labels[buildapi.BuildLabel], build.Name)
		return nil
	}
	err = bc.PodManager.DeletePod(build.Namespace, pod)
	if err != nil && !errors.IsNotFound(err) {
		glog.V(2).Infof("Failed to delete pod %s/%s for Build %s due to error: %v", build.Namespace, podName, build.Name, err)
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
