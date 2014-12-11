package controller

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	errors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// BuildController watches build resources and manages their state
type BuildController struct {
	BuildStore    cache.Store
	NextBuild     func() *buildapi.Build
	NextPod       func() *kapi.Pod
	BuildUpdater  buildUpdater
	PodManager    podManager
	BuildStrategy BuildStrategy
}

// BuildStrategy knows how to create a pod spec for a pod which can execute a build.
type BuildStrategy interface {
	CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error)
}

type buildUpdater interface {
	UpdateBuild(namespace string, build *buildapi.Build) (*buildapi.Build, error)
}

type podManager interface {
	CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	DeletePod(namespace string, pod *kapi.Pod) error
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

	nextStatus := buildapi.BuildStatusFailed
	build.PodName = fmt.Sprintf("build-%s", build.Name)

	// If a cancelling event was triggered for the build, update build status.
	if build.Cancelled {
		glog.V(2).Infof("Cancelling build %s.", build.Name)
		nextStatus = buildapi.BuildStatusCancelled

	} else {

		var podSpec *kapi.Pod
		var err error
		if podSpec, err = bc.BuildStrategy.CreateBuildPod(build); err != nil {
			glog.V(2).Infof("Strategy failed to create build pod definition: %v", err)
			nextStatus = buildapi.BuildStatusFailed
		} else {

			if _, err := bc.PodManager.CreatePod(build.Namespace, podSpec); err != nil {
				if !errors.IsAlreadyExists(err) {
					glog.V(2).Infof("Failed to create pod for build %s: %#v", build.Name, err)
					nextStatus = buildapi.BuildStatusFailed
				}
			} else {
				glog.V(2).Infof("Created build pod: %#v", podSpec)
				nextStatus = buildapi.BuildStatusPending
			}
		}
	}

	build.Status = nextStatus
	if _, err := bc.BuildUpdater.UpdateBuild(build.Namespace, build); err != nil {
		glog.V(2).Infof("Failed to update build %s: %#v", build.Name, err)
	}
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
		if _, err := bc.BuildUpdater.UpdateBuild(build.Namespace, build); err != nil {
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
	if _, err := bc.BuildUpdater.UpdateBuild(build.Namespace, build); err != nil {
		return err
	}

	glog.V(2).Infof("Build %s was successfully cancelled.", build.Name)
	return nil
}

// isBuildCancellable checks for build status and returns true if the condition is checked.
func isBuildCancellable(build *buildapi.Build) bool {
	return build.Status == buildapi.BuildStatusNew || build.Status == buildapi.BuildStatusPending || build.Status == buildapi.BuildStatusRunning
}
