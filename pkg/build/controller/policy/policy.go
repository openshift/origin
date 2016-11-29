package policy

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util/wait"
)

// RunPolicy is an interface that define handler for the build runPolicy field.
// The run policy controls how and when the new builds are 'run'.
type RunPolicy interface {
	// IsRunnable returns true of the given build should be executed.
	IsRunnable(*buildapi.Build) (bool, error)

	// OnComplete allows policy to execute action when the given build just
	// completed.
	OnComplete(*buildapi.Build) error
}

// GetAllRunPolicies returns a set of all run policies.
func GetAllRunPolicies(lister buildclient.BuildLister, updater buildclient.BuildUpdater) []RunPolicy {
	return []RunPolicy{
		&ParallelPolicy{BuildLister: lister, BuildUpdater: updater},
		&SerialPolicy{BuildLister: lister, BuildUpdater: updater},
		&SerialLatestOnlyPolicy{BuildLister: lister, BuildUpdater: updater},
	}
}

// ForBuild picks the appropriate run policy for the given build.
func ForBuild(build *buildapi.Build, policies []RunPolicy) RunPolicy {
	for _, s := range policies {
		switch buildutil.BuildRunPolicy(build) {
		case buildapi.BuildRunPolicyParallel:
			if _, ok := s.(*ParallelPolicy); ok {
				glog.V(5).Infof("Using %T run policy for build %s/%s", s, build.Namespace, build.Name)
				return s
			}
		case buildapi.BuildRunPolicySerial:
			if _, ok := s.(*SerialPolicy); ok {
				glog.V(5).Infof("Using %T run policy for build %s/%s", s, build.Namespace, build.Name)
				return s
			}
		case buildapi.BuildRunPolicySerialLatestOnly:
			if _, ok := s.(*SerialLatestOnlyPolicy); ok {
				glog.V(5).Infof("Using %T run policy for build %s/%s", s, build.Namespace, build.Name)
				return s
			}
		}
	}
	return nil
}

// hasRunningSerialBuild indicates that there is a running or pending serial
// build. This function is used to prevent running parallel builds because
// serial builds should always run alone.
func hasRunningSerialBuild(lister buildclient.BuildLister, namespace, buildConfigName string) bool {
	var hasRunningBuilds bool
	buildutil.BuildConfigBuilds(lister, namespace, buildConfigName, func(b buildapi.Build) bool {
		switch b.Status.Phase {
		case buildapi.BuildPhasePending, buildapi.BuildPhaseRunning:
			switch buildutil.BuildRunPolicy(&b) {
			case buildapi.BuildRunPolicySerial, buildapi.BuildRunPolicySerialLatestOnly:
				hasRunningBuilds = true
			}
		}
		return false
	})
	return hasRunningBuilds
}

// GetNextConfigBuild returns the build that will be executed next for the given
// build configuration. It also returns the indication whether there are
// currently running builds, to make sure there is no race-condition between
// re-listing the builds.
func GetNextConfigBuild(lister buildclient.BuildLister, namespace, buildConfigName string) ([]*buildapi.Build, bool, error) {
	var (
		nextBuild           *buildapi.Build
		hasRunningBuilds    bool
		previousBuildNumber int64
	)
	builds, err := buildutil.BuildConfigBuilds(lister, namespace, buildConfigName, func(b buildapi.Build) bool {
		switch b.Status.Phase {
		case buildapi.BuildPhasePending, buildapi.BuildPhaseRunning:
			hasRunningBuilds = true
			return false
		}
		// Only 'new' build can be scheduled to run next
		return b.Status.Phase == buildapi.BuildPhaseNew
	})
	if err != nil {
		return nil, hasRunningBuilds, err
	}

	nextParallelBuilds := []*buildapi.Build{}
	for i, b := range builds.Items {
		buildNumber, err := buildutil.BuildNumber(&b)
		if err != nil {
			return nil, hasRunningBuilds, err
		}
		if buildutil.BuildRunPolicy(&b) == buildapi.BuildRunPolicyParallel {
			nextParallelBuilds = append(nextParallelBuilds, &builds.Items[i])
		}
		if previousBuildNumber == 0 || buildNumber < previousBuildNumber {
			nextBuild = &builds.Items[i]
			previousBuildNumber = buildNumber
		}
	}
	nextBuilds := []*buildapi.Build{}
	// if the next build is a parallel build, then start all the queued parallel builds,
	// otherwise just start the next build if there is one.
	if nextBuild != nil && buildutil.BuildRunPolicy(nextBuild) == buildapi.BuildRunPolicyParallel {
		nextBuilds = nextParallelBuilds
	} else if nextBuild != nil {
		nextBuilds = append(nextBuilds, nextBuild)
	}
	return nextBuilds, hasRunningBuilds, nil
}

// handleComplete represents the default OnComplete handler. This Handler will
// check which build should be run next and update the StartTimestamp field for
// that build. That will trigger HandleBuild() to process that build immediately
// and as a result the build is immediately executed.
func handleComplete(lister buildclient.BuildLister, updater buildclient.BuildUpdater, build *buildapi.Build) error {
	bcName := buildutil.ConfigNameForBuild(build)
	if len(bcName) == 0 {
		return nil
	}
	nextBuilds, hasRunningBuilds, err := GetNextConfigBuild(lister, build.Namespace, bcName)
	if err != nil {
		return fmt.Errorf("unable to get the next build for %s/%s: %v", build.Namespace, build.Name, err)
	}
	if hasRunningBuilds || len(nextBuilds) == 0 {
		return nil
	}
	now := unversioned.Now()
	for _, build := range nextBuilds {
		build.Status.StartTimestamp = &now
		err := wait.Poll(500*time.Millisecond, 5*time.Second, func() (bool, error) {
			err := updater.Update(build.Namespace, build)
			if err != nil && errors.IsConflict(err) {
				glog.V(5).Infof("Error updating build %s/%s: %v (will retry)", build.Namespace, build.Name, err)
				return false, nil
			}
			return true, err
		})
		if err != nil {
			return err
		}
	}
	return nil
}
