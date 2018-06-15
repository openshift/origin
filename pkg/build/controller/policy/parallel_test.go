package policy

import (
	"testing"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

func TestParallelIsRunnableNewBuilds(t *testing.T) {
	allNewBuilds := []buildapi.Build{
		addBuild("build-1", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicyParallel),
		addBuild("build-2", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicyParallel),
		addBuild("build-3", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicyParallel),
	}
	client := newTestClient(allNewBuilds)
	policy := ParallelPolicy{BuildLister: client.Lister(), BuildUpdater: client}
	for _, build := range allNewBuilds {
		runnable, err := policy.IsRunnable(&build)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if !runnable {
			t.Errorf("expected build %s runnable, is not", build.Name)
		}
	}
}

func TestParallelIsRunnableMixedBuilds(t *testing.T) {
	mixedBuilds := []buildapi.Build{
		addBuild("build-4", "sample-bc", buildapi.BuildPhaseRunning, buildapi.BuildRunPolicyParallel),
		addBuild("build-6", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicyParallel),
		addBuild("build-5", "sample-bc", buildapi.BuildPhasePending, buildapi.BuildRunPolicyParallel),
	}
	client := newTestClient(mixedBuilds)
	policy := ParallelPolicy{BuildLister: client.Lister(), BuildUpdater: client}
	for _, build := range mixedBuilds {
		runnable, err := policy.IsRunnable(&build)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if !runnable {
			t.Errorf("expected build %s runnable, is not", build.Name)
		}
	}
}

func TestParallelIsRunnableWithSerialRunning(t *testing.T) {
	mixedBuilds := []buildapi.Build{
		addBuild("build-7", "sample-bc", buildapi.BuildPhaseRunning, buildapi.BuildRunPolicySerial),
		addBuild("build-8", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicyParallel),
		addBuild("build-9", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicyParallel),
	}
	client := newTestClient(mixedBuilds)
	policy := ParallelPolicy{BuildLister: client.Lister(), BuildUpdater: client}
	for _, build := range mixedBuilds {
		runnable, err := policy.IsRunnable(&build)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if runnable {
			t.Errorf("expected build %s as not runnable", build.Name)
		}
	}
}
