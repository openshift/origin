package policy

import (
	"time"

	"k8s.io/kubernetes/pkg/api/errors"

	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/golang/glog"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

// SerialLatestOnlyPolicy implements the RunPolicy interface. This variant of
// the serial build policy makes sure that builds are executed in same order as
// they were created, but when a new build is created, the previous, queued
// build is cancelled, always making the latest created build run as next. This
// will produce consistent results, but might not suit the CI/CD flow where user
// expect that every commit is built.
type SerialLatestOnlyPolicy struct {
	BuildUpdater buildclient.BuildUpdater
	BuildLister  buildclient.BuildLister
}

// IsRunnable implements the RunPolicy interface.
// Calling this function on a build mean that any previous build that is in
// 'new' phase will be automatically cancelled. This will also cancel any
// "serial" build (when you changed the build config run policy on-the-fly).
func (s *SerialLatestOnlyPolicy) IsRunnable(build *buildapi.Build) (bool, error) {
	bcName := buildutil.ConfigNameForBuild(build)
	if len(bcName) == 0 {
		return true, nil
	}
	if err := kerrors.NewAggregate(s.cancelPreviousBuilds(build)); err != nil {
		return false, err
	}
	nextBuilds, runningBuilds, err := GetNextConfigBuild(s.BuildLister, build.Namespace, bcName)
	if err != nil || runningBuilds {
		return false, err
	}
	return len(nextBuilds) == 1 && nextBuilds[0].Name == build.Name, err
}

// IsRunnable implements the Scheduler interface.
func (s *SerialLatestOnlyPolicy) OnComplete(build *buildapi.Build) error {
	return handleComplete(s.BuildLister, s.BuildUpdater, build)
}

// cancelPreviousBuilds cancels all queued builds that have the build sequence number
// lower than the given build. It retries the cancellation in case of conflict.
func (s *SerialLatestOnlyPolicy) cancelPreviousBuilds(build *buildapi.Build) []error {
	bcName := buildutil.ConfigNameForBuild(build)
	if len(bcName) == 0 {
		return []error{}
	}
	currentBuildNumber, err := buildutil.BuildNumber(build)
	if err != nil {
		return []error{NewNoBuildNumberAnnotationError(build)}
	}
	builds, err := buildutil.BuildConfigBuilds(s.BuildLister, build.Namespace, bcName, func(b buildapi.Build) bool {
		// Do not cancel the complete builds, builds that were already cancelled, or
		// running builds.
		if buildutil.IsBuildComplete(&b) || b.Status.Phase == buildapi.BuildPhaseRunning {
			return false
		}

		// Prevent race-condition when there is a newer build than this and we don't
		// want to cancel it. The HandleBuild() function that runs for that build
		// will cancel this build.
		buildNumber, _ := buildutil.BuildNumber(&b)
		return buildNumber < currentBuildNumber
	})
	if err != nil {
		return []error{err}
	}
	var result = []error{}
	for _, b := range builds.Items {
		err := wait.Poll(500*time.Millisecond, 5*time.Second, func() (bool, error) {
			b.Status.Cancelled = true
			err := s.BuildUpdater.Update(b.Namespace, &b)
			if err != nil && errors.IsConflict(err) {
				glog.V(5).Infof("Error cancelling build %s/%s: %v (will retry)", b.Namespace, b.Name, err)
				return false, nil
			}
			return true, err
		})
		if err != nil {
			result = append(result, err)
		}
	}
	return result
}
