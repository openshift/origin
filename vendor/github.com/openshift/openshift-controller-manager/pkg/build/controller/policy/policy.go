package policy

import (
	"fmt"
	"strconv"

	"k8s.io/klog"

	buildv1 "github.com/openshift/api/build/v1"
	v1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	buildlister "github.com/openshift/client-go/build/listers/build/v1"
	buildutil "github.com/openshift/openshift-controller-manager/pkg/build/buildutil"
)

// RunPolicy is an interface that define handler for the build runPolicy field.
// The run policy controls how and when the new builds are 'run'.
type RunPolicy interface {
	// IsRunnable returns true of the given build should be executed.
	IsRunnable(*buildv1.Build) (bool, error)

	// Handles returns true if the run policy handles a specific policy
	Handles(buildv1.BuildRunPolicy) bool
}

// GetAllRunPolicies returns a set of all run policies.
func GetAllRunPolicies(lister buildlister.BuildLister, updater v1.BuildsGetter) []RunPolicy {
	return []RunPolicy{
		&ParallelPolicy{BuildLister: lister},
		&SerialPolicy{BuildLister: lister},
		&SerialLatestOnlyPolicy{BuildLister: lister, BuildUpdater: updater},
	}
}

// ForBuild picks the appropriate run policy for the given build.
func ForBuild(build *buildv1.Build, policies []RunPolicy) RunPolicy {
	buildPolicy := buildRunPolicy(build)
	for _, s := range policies {
		if s.Handles(buildPolicy) {
			klog.V(5).Infof("Using %T run policy for build %s/%s", s, build.Namespace, build.Name)
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
	if _, err := buildutil.BuildConfigBuildsFromLister(lister, namespace, buildConfigName, func(b *buildv1.Build) bool {
		switch b.Status.Phase {
		case buildv1.BuildPhasePending, buildv1.BuildPhaseRunning:
			switch buildRunPolicy(b) {
			case buildv1.BuildRunPolicySerial, buildv1.BuildRunPolicySerialLatestOnly:
				hasRunningBuilds = true
			}
		}
		return false
	}); err != nil {
		klog.Errorf("Failed to list builds for %s/%s: %v", namespace, buildConfigName, err)
	}
	return hasRunningBuilds
}

// GetNextConfigBuild returns the build that will be executed next for the given
// build configuration. It also returns the indication whether there are
// currently running builds, to make sure there is no race-condition between
// re-listing the builds.
func GetNextConfigBuild(lister buildlister.BuildLister, namespace, buildConfigName string) ([]*buildv1.Build, bool, error) {
	var (
		nextBuild           *buildv1.Build
		hasRunningBuilds    bool
		previousBuildNumber int64
	)
	builds, err := buildutil.BuildConfigBuildsFromLister(lister, namespace, buildConfigName, func(b *buildv1.Build) bool {
		switch b.Status.Phase {
		case buildv1.BuildPhasePending, buildv1.BuildPhaseRunning:
			hasRunningBuilds = true
		case buildv1.BuildPhaseNew:
			return true
		}
		return false
	})
	if err != nil {
		return nil, hasRunningBuilds, err
	}

	nextParallelBuilds := []*buildv1.Build{}
	for i, b := range builds {
		buildNumber, err := buildNumber(b)
		if err != nil {
			return nil, hasRunningBuilds, err
		}
		if buildRunPolicy(b) == buildv1.BuildRunPolicyParallel {
			nextParallelBuilds = append(nextParallelBuilds, b)
		}
		if previousBuildNumber == 0 || buildNumber < previousBuildNumber {
			nextBuild = builds[i]
			previousBuildNumber = buildNumber
		}
	}
	nextBuilds := []*buildv1.Build{}
	// if the next build is a parallel build, then start all the queued parallel builds,
	// otherwise just start the next build if there is one.
	if nextBuild != nil && buildRunPolicy(nextBuild) == buildv1.BuildRunPolicyParallel {
		nextBuilds = nextParallelBuilds
	} else if nextBuild != nil {
		nextBuilds = append(nextBuilds, nextBuild)
	}
	return nextBuilds, hasRunningBuilds, nil
}

// buildNumber returns the given build number.
func buildNumber(build *buildv1.Build) (int64, error) {
	annotations := build.GetAnnotations()
	if stringNumber, ok := annotations[buildv1.BuildNumberAnnotation]; ok {
		return strconv.ParseInt(stringNumber, 10, 64)
	}
	return 0, fmt.Errorf("build %s/%s does not have %s annotation", build.Namespace, build.Name, buildv1.BuildNumberAnnotation)
}

// buildRunPolicy returns the scheduling policy for the build based on the "queued" label.
func buildRunPolicy(build *buildv1.Build) buildv1.BuildRunPolicy {
	labels := build.GetLabels()
	if value, found := labels[buildv1.BuildRunPolicyLabel]; found {
		switch value {
		case "Parallel":
			return buildv1.BuildRunPolicyParallel
		case "Serial":
			return buildv1.BuildRunPolicySerial
		case "SerialLatestOnly":
			return buildv1.BuildRunPolicySerialLatestOnly
		}
	}
	klog.V(5).Infof("Build %s/%s does not have start policy label set, using default (Serial)", build.Namespace, build.Name)
	return buildv1.BuildRunPolicySerial
}
