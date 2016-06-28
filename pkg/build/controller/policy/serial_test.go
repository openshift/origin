package policy

import (
	"testing"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

func TestSerialIsRunnableNewBuilds(t *testing.T) {
	allNewBuilds := []buildapi.Build{
		addBuild("build-1", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicySerial),
		addBuild("build-2", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicySerial),
		addBuild("build-3", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicySerial),
	}
	client := newTestClient(allNewBuilds)
	policy := SerialPolicy{BuildLister: client, BuildUpdater: client}
	runnableBuilds := []string{
		"build-1",
	}
	shouldRun := func(name string) bool {
		for _, b := range runnableBuilds {
			if b == name {
				return true
			}
		}
		return false
	}
	shouldNotRun := func(name string) bool {
		return !shouldRun(name)
	}
	for _, build := range allNewBuilds {
		runnable, err := policy.IsRunnable(&build)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if runnable && shouldNotRun(build.Name) {
			t.Errorf("%s should not be runnable", build.Name)
		}
		if !runnable && shouldRun(build.Name) {
			t.Errorf("%s should be runnable, it is not", build.Name)
		}
	}
}
