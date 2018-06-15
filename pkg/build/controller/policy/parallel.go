package policy

import (
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildlister "github.com/openshift/origin/pkg/build/generated/listers/build/internalversion"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

// ParallelPolicy implements the RunPolicy interface. Build created using this
// run policy will always run as soon as they are created.
// This run policy does not guarantee that the builds will complete in same
// order as they were created and using this policy might cause unpredictable
// behavior.
type ParallelPolicy struct {
	BuildLister  buildlister.BuildLister
	BuildUpdater buildclient.BuildUpdater
}

// IsRunnable implements the RunPolicy interface. The parallel builds are run as soon
// as they are created. There is no build queue as all build run asynchronously.
func (s *ParallelPolicy) IsRunnable(build *buildapi.Build) (bool, error) {
	bcName := buildutil.ConfigNameForBuild(build)
	if len(bcName) == 0 {
		return true, nil
	}
	return !hasRunningSerialBuild(s.BuildLister, build.Namespace, bcName), nil
}

// Handles returns true for the build run parallel policy
func (s *ParallelPolicy) Handles(policy buildapi.BuildRunPolicy) bool {
	return policy == buildapi.BuildRunPolicyParallel
}
