package controller

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	errors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
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
}

// BuildStrategy knows how to create a pod spec for a pod which can execute a build.
type BuildStrategy interface {
	CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error)
}

type podManager interface {
	CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	DeletePod(namespace string, pod *kapi.Pod) error
}

type imageStreamClient interface {
	GetImageStream(namespace, name string) (*imageapi.ImageStream, error)
}

func (bc *BuildController) HandleBuild(build *buildapi.Build) error {
	glog.V(4).Infof("Handling build %s", build.Name)

	// We only deal with new builds here
	if build.Status != buildapi.BuildStatusNew {
		return nil
	}

	if err := bc.nextBuildStatus(build); err != nil {
		// TODO: all build errors should be retried, and build error should not be a permanent status change.
		// Instead, we should requeue this build request using the same backoff logic as the scheduler.
		//build.Status = buildapi.BuildStatusError
		//build.Message = err.Error()
		return fmt.Errorf("Build failed with error %s/%s: %#v", build.Namespace, build.Name, err)
	}

	if err := bc.BuildUpdater.Update(build.Namespace, build); err != nil {
		// This is not a retryable error because the build has been created.  The worst case
		// outcome of not updating the buildconfig is that we might rerun a build for the
		// same "new" imageid change in the future, which is better than guaranteeing we
		// run the build 2+ times by retrying it here.
		glog.V(2).Infof("Failed to record changes to build %s/%s: %#v", build.Namespace, build.Name, err)
	}
	return nil
}

// nextBuildStatus updates build with any appropriate changes, or returns an error if
// the change cannot occur. When returning nil, be sure to set build.Status and optionally
// build.Message.
func (bc *BuildController) nextBuildStatus(build *buildapi.Build) error {
	// If a cancelling event was triggered for the build, update build status.
	if build.Cancelled {
		glog.V(4).Infof("Cancelling build %s.", build.Name)
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
				return fmt.Errorf("the referenced output image repository %s/%s does not exist", namespace, ref.Name)
			}
			return fmt.Errorf("the referenced output repo %s/%s could not be found by %s/%s: %v", namespace, ref.Name, build.Namespace, build.Name, err)
		}
		if len(build.Parameters.Output.Tag) == 0 {
			spec = repo.Status.DockerImageRepository
		} else {
			spec = fmt.Sprintf("%s:%s", repo.Status.DockerImageRepository, build.Parameters.Output.Tag)
		}
	}

	// set the expected build parameters, which will be saved if no error occurs
	build.Status = buildapi.BuildStatusPending
	// override DockerImageReference in the strategy for the copy we send to the server
	build.Parameters.Output.DockerImageReference = spec

	copy, err := kapi.Scheme.Copy(build)
	if err != nil {
		return fmt.Errorf("unable to copy build: %v", err)
	}
	buildCopy := copy.(*buildapi.Build)

	// invoke the strategy to get a build pod
	podSpec, err := bc.BuildStrategy.CreateBuildPod(buildCopy)
	if err != nil {
		return fmt.Errorf("the strategy failed to create a build pod for %s/%s: %v", build.Namespace, build.Name, err)
	}

	if _, err := bc.PodManager.CreatePod(build.Namespace, podSpec); err != nil {
		if errors.IsAlreadyExists(err) {
			glog.V(4).Infof("Build pod already existed: %#v", podSpec)
			return nil
		}
		return fmt.Errorf("failed to create pod for build %s/%s: %v", build.Namespace, build.Name, err)
	}

	glog.V(4).Infof("Created pod for build: %#v", podSpec)
	return nil
}

// BuildPodController watches pods running builds and manages the build state
type BuildPodController struct {
	BuildStore   cache.Store
	BuildUpdater buildclient.BuildUpdater
	PodManager   podManager
}

func (bc *BuildPodController) HandlePod(pod *kapi.Pod) error {
	// Find the build for this pod
	var build *buildapi.Build
	for _, obj := range bc.BuildStore.List() {
		b := obj.(*buildapi.Build)
		if buildutil.GetBuildPodName(b) == pod.Name {
			build = b
			break
		}
	}

	if build == nil {
		return nil
	}

	// A cancelling event was triggered for the build, delete its pod and update build status.
	if build.Cancelled {
		glog.V(2).Infof("Cancelling build %s.", build.Name)

		if err := bc.CancelBuild(build, pod); err != nil {
			return fmt.Errorf("Failed to cancel build %s: %#v, will retry", build.Name, err)
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
			if info.State.Termination != nil && info.State.Termination.ExitCode != 0 {
				nextStatus = buildapi.BuildStatusFailed
				break
			}
		}
	}

	if build.Status != nextStatus {
		glog.V(4).Infof("Updating build %s status %s -> %s", build.Name, build.Status, nextStatus)
		build.Status = nextStatus
		if build.Status == buildapi.BuildStatusComplete || build.Status == buildapi.BuildStatusFailed || build.Status == buildapi.BuildStatusCancelled {
			dummy := util.Now()
			build.CompletionTimestamp = &dummy
		}
		if build.Status == buildapi.BuildStatusRunning {
			dummy := util.Now()
			build.StartTimestamp = &dummy
		}
		if err := bc.BuildUpdater.Update(build.Namespace, build); err != nil {
			return fmt.Errorf("Failed to update build %s: %#v", build.Name, err)
		}
	}
	return nil
}

// CancelBuild updates a build status to Cancelled, after its associated pod is deleted.
func (bc *BuildPodController) CancelBuild(build *buildapi.Build, pod *kapi.Pod) error {
	if !isBuildCancellable(build) {
		glog.V(2).Infof("The build can be cancelled only if it has pending/running status, not %s.", build.Status)
		return nil
	}

	err := bc.PodManager.DeletePod(build.Namespace, pod)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	build.Status = buildapi.BuildStatusCancelled
	dummy := util.Now()
	build.CompletionTimestamp = &dummy
	if err := bc.BuildUpdater.Update(build.Namespace, build); err != nil {
		return err
	}

	glog.V(2).Infof("Build %s was successfully cancelled.", build.Name)
	return nil
}

// isBuildCancellable checks for build status and returns true if the condition is checked.
func isBuildCancellable(build *buildapi.Build) bool {
	return build.Status == buildapi.BuildStatusNew || build.Status == buildapi.BuildStatusPending || build.Status == buildapi.BuildStatusRunning
}
