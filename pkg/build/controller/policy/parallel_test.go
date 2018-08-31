package policy

import (
	"testing"

	buildv1 "github.com/openshift/api/build/v1"
)

func TestParallelIsRunnableNewBuilds(t *testing.T) {
	allNewBuilds := []buildv1.Build{
		addBuild("build-1", "sample-bc", buildv1.BuildPhaseNew, buildv1.BuildRunPolicyParallel),
		addBuild("build-2", "sample-bc", buildv1.BuildPhaseNew, buildv1.BuildRunPolicyParallel),
		addBuild("build-3", "sample-bc", buildv1.BuildPhaseNew, buildv1.BuildRunPolicyParallel),
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
	mixedBuilds := []buildv1.Build{
		addBuild("build-4", "sample-bc", buildv1.BuildPhaseRunning, buildv1.BuildRunPolicyParallel),
		addBuild("build-6", "sample-bc", buildv1.BuildPhaseNew, buildv1.BuildRunPolicyParallel),
		addBuild("build-5", "sample-bc", buildv1.BuildPhasePending, buildv1.BuildRunPolicyParallel),
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
	mixedBuilds := []buildv1.Build{
		addBuild("build-7", "sample-bc", buildv1.BuildPhaseRunning, buildv1.BuildRunPolicySerial),
		addBuild("build-8", "sample-bc", buildv1.BuildPhaseNew, buildv1.BuildRunPolicyParallel),
		addBuild("build-9", "sample-bc", buildv1.BuildPhaseNew, buildv1.BuildRunPolicyParallel),
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
