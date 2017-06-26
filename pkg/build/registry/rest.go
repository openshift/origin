package registry

import (
	"fmt"
	"time"

	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

var (
	// ErrUnknownBuildPhase is returned for WaitForRunningBuild if an unknown phase is returned.
	ErrUnknownBuildPhase = fmt.Errorf("unknown build phase")
	ErrBuildDeleted      = fmt.Errorf("build was deleted")
)

// WaitForRunningBuild waits until the specified build is no longer New or Pending. Returns true if
// the build ran within timeout, false if it did not, and an error if any other error state occurred.
// The last observed Build state is returned.
func WaitForRunningBuild(watcher rest.Watcher, ctx apirequest.Context, build *buildapi.Build, timeout time.Duration) (*buildapi.Build, bool, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", build.Name)
	options := &metainternal.ListOptions{FieldSelector: fieldSelector, ResourceVersion: build.ResourceVersion}
	w, err := watcher.Watch(ctx, options)
	if err != nil {
		return build, false, err
	}
	defer w.Stop()

	observed := build
	ch := w.ResultChan()
	expire := time.After(timeout)
	for {
		select {
		case event := <-ch:
			obj, ok := event.Object.(*buildapi.Build)
			if !ok {
				return observed, false, fmt.Errorf("received unknown object while watching for builds")
			}
			observed = obj

			if event.Type == watch.Deleted {
				return observed, false, ErrBuildDeleted
			}
			switch obj.Status.Phase {
			case buildapi.BuildPhaseRunning, buildapi.BuildPhaseComplete, buildapi.BuildPhaseFailed, buildapi.BuildPhaseError, buildapi.BuildPhaseCancelled:
				return observed, true, nil
			case buildapi.BuildPhaseNew, buildapi.BuildPhasePending:
			default:
				return observed, false, ErrUnknownBuildPhase
			}
		case <-expire:
			return observed, false, nil
		}
	}
}
