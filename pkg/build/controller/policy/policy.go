package policy

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildlister "github.com/openshift/origin/pkg/build/generated/listers/build/internalversion"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

// RunPolicy is an interface that define handler for the build runPolicy field.
// The run policy controls how and when the new builds are 'run'.
type RunPolicy interface {
	// IsRunnable returns true of the given build should be executed.
	IsRunnable(*buildapi.Build) (bool, error)

	// Handles returns true if the run policy handles a specific policy
	Handles(buildapi.BuildRunPolicy) bool
}

// GetAllRunPolicies returns a set of all run policies.
func GetAllRunPolicies(lister buildlister.BuildLister, updater buildclient.BuildUpdater) []RunPolicy {
	return []RunPolicy{
		&ParallelPolicy{BuildLister: lister, BuildUpdater: updater},
		&SerialPolicy{BuildLister: lister, BuildUpdater: updater},
		&SerialLatestOnlyPolicy{BuildLister: lister, BuildUpdater: updater},
	}
}

// ForBuild picks the appropriate run policy for the given build.
func ForBuild(build *buildapi.Build, policies []RunPolicy) RunPolicy {
	buildPolicy := buildRunPolicy(build)
	for _, s := range policies {
		if s.Handles(buildPolicy) {
			glog.V(5).Infof("Using %T run policy for build %s/%s", s, build.Namespace, build.Name)
			return s
		}
	}
	return nil
}

// hasRunningSerialBuild indicates that there is a running or pending serial
// build. This function is used to prevent running parallel builds because
// serial builds should always run alone.
func hasRunningSerialBuild(lister buildlister.BuildLister, namespace, buildConfigName string) bool {
	var hasRunningBuilds bool
	buildutil.BuildConfigBuilds(lister, namespace, buildConfigName, func(b *buildapi.Build) bool {
		switch b.Status.Phase {
		case buildapi.BuildPhasePending, buildapi.BuildPhaseRunning:
			switch buildRunPolicy(b) {
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
func GetNextConfigBuild(lister buildlister.BuildLister, namespace, buildConfigName string) ([]*buildapi.Build, bool, error) {
	var (
		nextBuild           *buildapi.Build
		hasRunningBuilds    bool
		previousBuildNumber int64
	)
	builds, err := buildutil.BuildConfigBuilds(lister, namespace, buildConfigName, func(b *buildapi.Build) bool {
		switch b.Status.Phase {
		case buildapi.BuildPhasePending, buildapi.BuildPhaseRunning:
			hasRunningBuilds = true
		case buildapi.BuildPhaseNew:
			return true
		}
		return false
	})
	if err != nil {
		return nil, hasRunningBuilds, err
	}

	nextParallelBuilds := []*buildapi.Build{}
	for i, b := range builds {
		buildNumber, err := buildNumber(b)
		if err != nil {
			return nil, hasRunningBuilds, err
		}
		if buildRunPolicy(b) == buildapi.BuildRunPolicyParallel {
			nextParallelBuilds = append(nextParallelBuilds, b)
		}
		if previousBuildNumber == 0 || buildNumber < previousBuildNumber {
			nextBuild = builds[i]
			previousBuildNumber = buildNumber
		}
	}
	nextBuilds := []*buildapi.Build{}
	// if the next build is a parallel build, then start all the queued parallel builds,
	// otherwise just start the next build if there is one.
	if nextBuild != nil && buildRunPolicy(nextBuild) == buildapi.BuildRunPolicyParallel {
		nextBuilds = nextParallelBuilds
	} else if nextBuild != nil {
		nextBuilds = append(nextBuilds, nextBuild)
	}
	return nextBuilds, hasRunningBuilds, nil
}

// buildNumber returns the given build number.
func buildNumber(build *buildapi.Build) (int64, error) {
	annotations := build.GetAnnotations()
	if stringNumber, ok := annotations[buildapi.BuildNumberAnnotation]; ok {
		return strconv.ParseInt(stringNumber, 10, 64)
	}
	return 0, fmt.Errorf("build %s/%s does not have %s annotation", build.Namespace, build.Name, buildapi.BuildNumberAnnotation)
}

// buildRunPolicy returns the scheduling policy for the build based on the "queued" label.
func buildRunPolicy(build *buildapi.Build) buildapi.BuildRunPolicy {
	labels := build.GetLabels()
	if value, found := labels[buildapi.BuildRunPolicyLabel]; found {
		switch value {
		case "Parallel":
			return buildapi.BuildRunPolicyParallel
		case "Serial":
			return buildapi.BuildRunPolicySerial
		case "SerialLatestOnly":
			return buildapi.BuildRunPolicySerialLatestOnly
		}
	}
	glog.V(5).Infof("Build %s/%s does not have start policy label set, using default (Serial)", build.Namespace, build.Name)
	return buildapi.BuildRunPolicySerial
}
