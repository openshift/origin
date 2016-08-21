package policy

import (
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

// SerialPolicy implements the RunPolicy interface. Using this run policy, every
// created build is put into a queue. The serial run policy guarantees that
// all builds are executed synchroniously in the same order as they were
// created. This will produce consistent results, but block the build execution until the
// previous builds are complete.
type SerialPolicy struct {
	BuildLister  buildclient.BuildLister
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

// OnComplete implements the RunPolicy interface.
func (s *SerialPolicy) OnComplete(build *buildapi.Build) error {
	return handleComplete(s.BuildLister, s.BuildUpdater, build)
}
