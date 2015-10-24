package registry

import (
	"fmt"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/build/api"
)

// ErrUnknownBuildPhase is returned for WaitForRunningBuild if an unknown phase is returned.
var ErrUnknownBuildPhase = fmt.Errorf("unknown build phase")

// WaitForRunningBuild waits until the specified build is no longer New or Pending. Returns true if
// the build ran within timeout, false if it did not, and an error if any other error state occurred.
// The last observed Build state is returned.
func WaitForRunningBuild(watcher rest.Watcher, ctx kapi.Context, build *api.Build, timeout time.Duration) (*api.Build, bool, error) {
	fieldSelector := fields.Set{"metadata.name": build.Name}.AsSelector()
	w, err := watcher.Watch(ctx, labels.Everything(), fieldSelector, build.ResourceVersion)
	if err != nil {
		return nil, false, err
	}
	defer w.Stop()

	ch := w.ResultChan()
	observed := build
	expire := time.After(timeout)
	for {
		select {
		case event := <-ch:
			obj, ok := event.Object.(*api.Build)
			if !ok {
				return observed, false, fmt.Errorf("received unknown object while watching for builds")
			}
			observed = obj

			switch obj.Status.Phase {
			case api.BuildPhaseRunning, api.BuildPhaseComplete, api.BuildPhaseFailed, api.BuildPhaseError, api.BuildPhaseCancelled:
				return observed, true, nil
			case api.BuildPhaseNew, api.BuildPhasePending:
			default:
				return observed, false, ErrUnknownBuildPhase
			}
		case <-expire:
			return observed, false, nil
		}
	}
}
