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
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// BuildController watches build resources and manages their state
type BuildController struct {
	BuildStore    cache.Store
	NextBuild     func() *buildapi.Build
	NextPod       func() *kapi.Pod
	BuildUpdater  buildclient.BuildUpdater
	PodManager    podManager
	BuildStrategy BuildStrategy

	ImageRepositoryClient imageRepositoryClient
}

// BuildStrategy knows how to create a pod spec for a pod which can execute a build.
type BuildStrategy interface {
	CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error)
}

type podManager interface {
	CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	DeletePod(namespace string, pod *kapi.Pod) error
}

type imageRepositoryClient interface {
	GetImageRepository(namespace, name string) (*imageapi.ImageRepository, error)
}

// Run begins watching and syncing build jobs onto the cluster.
func (bc *BuildController) Run() {
	go util.Forever(func() { bc.HandleBuild(bc.NextBuild()) }, 0)
	go util.Forever(func() { bc.HandlePod(bc.NextPod()) }, 0)
}

func (bc *BuildController) HandleBuild(build *buildapi.Build) {
	glog.V(4).Infof("Handling build %s", build.Name)

	// We only deal with new builds here
	if build.Status != buildapi.BuildStatusNew {
		return
	}

	if err := bc.nextBuildStatus(build); err != nil {
		// TODO: all build errors should be retried, and build error should not be a permanent status change.
		// Instead, we should requeue this build request using the same backoff logic as the scheduler.
		// BuildStatusError should be reserved for meaning "permanently errored, no way to try again".
		glog.V(4).Infof("Build failed with error %s/%s: %#v", build.Namespace, build.Name, err)
		build.Status = buildapi.BuildStatusError
		build.Message = err.Error()
	}

	if err := bc.BuildUpdater.Update(build.Namespace, build); err != nil {
		glog.V(2).Infof("Failed to record changes to build %s/%s: %#v", build.Namespace, build.Name, err)
	}
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

		repo, err := bc.ImageRepositoryClient.GetImageRepository(namespace, ref.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("the referenced output image repository %s/%s does not exist", namespace, ref.Name)
			}
			return fmt.Errorf("the referenced output repo %s/%s could not be found by %s/%s: %v", namespace, ref.Name, build.Namespace, build.Name, err)
		}
		spec = repo.Status.DockerImageRepository
	}

	// set the expected build parameters, which will be saved if no error occurs
	build.Status = buildapi.BuildStatusPending
	build.PodName = fmt.Sprintf("build-%s", build.Name)

	// override DockerImageReference in the strategy for the copy we send to the server
	copy, err := kapi.Scheme.Copy(build)
	if err != nil {
		return fmt.Errorf("unable to copy build: %v", err)
	}
	buildCopy := copy.(*buildapi.Build)
	buildCopy.Parameters.Output.DockerImageReference = spec

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
		return fmt.Errorf("failed to create pod for build %s/%s: s", build.Namespace, build.Name, err)
	}

	glog.V(4).Infof("Created pod for build: %#v", podSpec)
	return nil
}

func (bc *BuildController) HandlePod(pod *kapi.Pod) {
	// Find the build for this pod
	var build *buildapi.Build
	for _, obj := range bc.BuildStore.List() {
		b := obj.(*buildapi.Build)
		if b.PodName == pod.Name {
			build = b
			break
		}
	}

	if build == nil {
		return
	}

	// A cancelling event was triggered for the build, delete its pod and update build status.
	if build.Cancelled {
		glog.V(2).Infof("Cancelling build %s.", build.Name)

		if err := bc.CancelBuild(build, pod); err != nil {
			glog.Errorf("Failed to cancel build %s: %#v", build.Name, err)
		}
		return
	}

	nextStatus := build.Status

	switch pod.Status.Phase {
	case kapi.PodRunning:
		// The pod's still running
		nextStatus = buildapi.BuildStatusRunning
	case kapi.PodSucceeded, kapi.PodFailed:
		// Check the exit codes of all the containers in the pod
		nextStatus = buildapi.BuildStatusComplete
		for _, info := range pod.Status.Info {
			if info.State.Termination != nil && info.State.Termination.ExitCode != 0 {
				nextStatus = buildapi.BuildStatusFailed
				break
			}
		}
	}

	if build.Status != nextStatus {
		glog.V(4).Infof("Updating build %s status %s -> %s", build.Name, build.Status, nextStatus)
		build.Status = nextStatus
		if err := bc.BuildUpdater.Update(build.Namespace, build); err != nil {
			glog.Errorf("Failed to update build %s: %#v", build.Name, err)
		}
	}
}

// CancelBuild updates a build status to Cancelled, after its associated pod is associated.
func (bc *BuildController) CancelBuild(build *buildapi.Build, pod *kapi.Pod) error {
	if !isBuildCancellable(build) {
		glog.V(2).Infof("The build can be cancelled only if it has pending/running status, not %s.", build.Status)
		return nil
	}

	err := bc.PodManager.DeletePod(build.Namespace, pod)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	build.Status = buildapi.BuildStatusCancelled
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
