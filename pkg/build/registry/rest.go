package registry

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildtypedclient "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
)

var (
	// ErrUnknownBuildPhase is returned for WaitForRunningBuild if an unknown phase is returned.
	ErrUnknownBuildPhase = fmt.Errorf("unknown build phase")
	ErrBuildDeleted      = fmt.Errorf("build was deleted")
)

type ErrWatchError struct {
	error
}

// WaitForRunningBuild waits until the specified build is no longer New or Pending. Returns true if
// the build ran within timeout, false if it did not, and an error if any other error state occurred.
// The last observed Build state is returned.
func WaitForRunningBuild(buildClient buildtypedclient.BuildsGetter, build *buildapi.Build, timeout time.Duration) (*buildapi.Build, bool, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", build.Name)
	options := metav1.ListOptions{FieldSelector: fieldSelector.String(), ResourceVersion: "0"}

	done := make(chan interface{}, 1)
	var resultBuild *buildapi.Build
	var success bool
	var resultErr error

	deadline := time.Now().Add(timeout)
	go func() {
		defer close(done)
		defer utilruntime.HandleCrash()

		for time.Now().Before(deadline) {

			// make sure the build has not been deleted before we start trying to watch on it because
			// we won't get a watch event for it if it's been deleted (because we are watching starting
			// at resource version 0).
			_, err := buildClient.Builds(build.Namespace).Get(build.Name, metav1.GetOptions{})
			if err != nil {
				resultErr = err
				if errors.IsNotFound(err) {
					resultErr = ErrBuildDeleted
				}
				return
			}

			w, err := buildClient.Builds(build.Namespace).Watch(options)
			if err != nil {
				resultErr = err
				return
			}

			_, err = watch.Until(timeout, w, func(event watch.Event) (bool, error) {
				if event.Type == watch.Error {
					return false, ErrWatchError{fmt.Errorf("watch event type error: %v", event)}
				}
				obj, ok := event.Object.(*buildapi.Build)
				if !ok {
					return false, fmt.Errorf("received unknown object while watching for builds: %T", event.Object)
				}

				if event.Type == watch.Deleted {
					return false, ErrBuildDeleted
				}

				switch obj.Status.Phase {
				case buildapi.BuildPhaseRunning, buildapi.BuildPhaseComplete, buildapi.BuildPhaseFailed, buildapi.BuildPhaseError, buildapi.BuildPhaseCancelled:
					resultBuild = obj
					return true, nil
				case buildapi.BuildPhaseNew, buildapi.BuildPhasePending:
				default:
					return false, ErrUnknownBuildPhase
				}

				return false, nil
			})

			if err != nil {
				if _, ok := err.(ErrWatchError); ok {
					continue
				}
				resultErr = err
				success = false
				resultBuild = nil
				return
			}
			success = true
			return
		}
	}()

	select {
	case <-time.After(timeout):
		return nil, false, wait.ErrWaitTimeout
	case <-done:
		return resultBuild, success, resultErr
	}

}
