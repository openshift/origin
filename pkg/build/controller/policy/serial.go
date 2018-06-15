package policy

import (
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildlister "github.com/openshift/origin/pkg/build/generated/listers/build/internalversion"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

// SerialPolicy implements the RunPolicy interface. Using this run policy, every
// created build is put into a queue. The serial run policy guarantees that
// all builds are executed synchroniously in the same order as they were
// created. This will produce consistent results, but block the build execution until the
// previous builds are complete.
type SerialPolicy struct {
	BuildLister  buildlister.BuildLister
	BuildUpdater buildclient.BuildUpdater
}

// IsRunnable implements the RunPolicy interface.
func (s *SerialPolicy) IsRunnable(build *buildapi.Build) (bool, error) {
	bcName := buildutil.ConfigNameForBuild(build)
	if len(bcName) == 0 {
		return true, nil
	}
	nextBuilds, runningBuilds, err := GetNextConfigBuild(s.BuildLister, build.Namespace, bcName)
	if err != nil || runningBuilds {
		return false, err
	}
	return len(nextBuilds) == 1 && nextBuilds[0].Name == build.Name, err
}

// Handles returns true for the build run serial policy
func (s *SerialPolicy) Handles(policy buildapi.BuildRunPolicy) bool {
	return policy == buildapi.BuildRunPolicySerial
}
