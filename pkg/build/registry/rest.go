package registry

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildtypedclient "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
)

var (
	// ErrUnknownBuildPhase is returned for WaitForRunningBuild if an unknown phase is returned.
	ErrUnknownBuildPhase = fmt.Errorf("unknown build phase")
	ErrBuildDeleted      = fmt.Errorf("build was deleted")
)

// WaitForRunningBuild waits until the specified build is no longer New or Pending. Returns true if
// the build ran within timeout, false if it did not, and an error if any other error state occurred.
// The last observed Build state is returned.
func WaitForRunningBuild(buildClient buildtypedclient.BuildsGetter, build *buildapi.Build, timeout time.Duration) (*buildapi.Build, bool, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", build.Name)
	options := metav1.ListOptions{FieldSelector: fieldSelector.String(), ResourceVersion: build.ResourceVersion}
	w, err := buildClient.Builds(build.Namespace).Watch(options)
	if err != nil {
		return build, false, err
	}

	observed := build
	_, err = watch.Until(timeout, w, func(event watch.Event) (bool, error) {
		obj, ok := event.Object.(*buildapi.Build)
		if !ok {
			return false, fmt.Errorf("received unknown object while watching for builds: %T", event.Object)
		}
		observed = obj

		if event.Type == watch.Deleted {
			return false, ErrBuildDeleted
		}
		switch obj.Status.Phase {
		case buildapi.BuildPhaseRunning, buildapi.BuildPhaseComplete, buildapi.BuildPhaseFailed, buildapi.BuildPhaseError, buildapi.BuildPhaseCancelled:
			return true, nil
		case buildapi.BuildPhaseNew, buildapi.BuildPhasePending:
		default:
			return false, ErrUnknownBuildPhase
		}

		return false, nil
	})
	if err != nil {
		return nil, false, err
	}

	return observed, true, nil
}
