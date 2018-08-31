package policy

import (
	buildv1 "github.com/openshift/api/build/v1"
	buildlister "github.com/openshift/client-go/build/listers/build/v1"
	buildclient "github.com/openshift/origin/pkg/build/client"
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
func (s *SerialPolicy) IsRunnable(build *buildv1.Build) (bool, error) {
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
func (s *SerialPolicy) Handles(policy buildv1.BuildRunPolicy) bool {
	return policy == buildv1.BuildRunPolicySerial
}
