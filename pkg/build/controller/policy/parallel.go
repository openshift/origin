package policy

import (
	buildv1 "github.com/openshift/api/build/v1"
	buildlister "github.com/openshift/client-go/build/listers/build/v1"
	buildclient "github.com/openshift/origin/pkg/build/client"
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
func (s *ParallelPolicy) IsRunnable(build *buildv1.Build) (bool, error) {
	bcName := buildutil.ConfigNameForBuild(build)
	if len(bcName) == 0 {
		return true, nil
	}
	return !hasRunningSerialBuild(s.BuildLister, build.Namespace, bcName), nil
}

// Handles returns true for the build run parallel policy
func (s *ParallelPolicy) Handles(policy buildv1.BuildRunPolicy) bool {
	return policy == buildv1.BuildRunPolicyParallel
}
