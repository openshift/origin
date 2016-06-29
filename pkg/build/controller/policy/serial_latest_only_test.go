package policy

import (
	"testing"

	buildapi "github.com/openshift/origin/pkg/build/api"
	kapi "k8s.io/kubernetes/pkg/api"
)

func TestSerialLatestOnlyIsRunnableNewBuilds(t *testing.T) {
	allNewBuilds := []buildapi.Build{
		addBuild("build-1", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicySerialLatestOnly),
		addBuild("build-2", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicySerialLatestOnly),
		addBuild("build-3", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicySerialLatestOnly),
	}
	client := newTestClient(allNewBuilds)
	policy := SerialLatestOnlyPolicy{BuildLister: client, BuildUpdater: client}
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
	builds, err := client.List("test", kapi.ListOptions{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !builds.Items[1].Status.Cancelled {
		t.Errorf("expected build-2 to be cancelled")
	}
}

func TestSerialLatestOnlyIsRunnableMixed(t *testing.T) {
	allNewBuilds := []buildapi.Build{
		addBuild("build-1", "sample-bc", buildapi.BuildPhaseComplete, buildapi.BuildRunPolicySerialLatestOnly),
		addBuild("build-2", "sample-bc", buildapi.BuildPhaseCancelled, buildapi.BuildRunPolicySerialLatestOnly),
		addBuild("build-3", "sample-bc", buildapi.BuildPhaseRunning, buildapi.BuildRunPolicySerialLatestOnly),
		addBuild("build-4", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicySerialLatestOnly),
	}
	client := newTestClient(allNewBuilds)
	policy := SerialLatestOnlyPolicy{BuildLister: client, BuildUpdater: client}
	for _, build := range allNewBuilds {
		runnable, err := policy.IsRunnable(&build)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if runnable {
			t.Errorf("%s should not be runnable", build.Name)
		}
	}
	builds, err := client.List("test", kapi.ListOptions{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if builds.Items[0].Status.Cancelled {
		t.Errorf("expected build-1 is complete and should not be cancelled")
	}
	if builds.Items[2].Status.Cancelled {
		t.Errorf("expected build-3 is running and should not be cancelled")
	}
	if builds.Items[3].Status.Cancelled {
		t.Errorf("expected build-4 will run next and should not be cancelled")
	}
}

func TestSerialLatestOnlyIsRunnableBuildsWithErrors(t *testing.T) {
	builds := []buildapi.Build{
		addBuild("build-1", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicySerialLatestOnly),
		addBuild("build-2", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicySerialLatestOnly),
	}

	// The build-1 will lack required labels
	builds[0].ObjectMeta.Labels = map[string]string{}

	// The build-2 will lack the build number annotation
	builds[1].ObjectMeta.Annotations = map[string]string{}

	client := newTestClient(builds)
	policy := SerialLatestOnlyPolicy{BuildLister: client, BuildUpdater: client}

	ok, err := policy.IsRunnable(&builds[0])
	if !ok || err != nil {
		t.Errorf("expected build to be runnable, got %v, error: %v", ok, err)
	}

	// No type-check as this error is returned as kerrors.aggregate
	if _, err := policy.IsRunnable(&builds[1]); err == nil {
		t.Errorf("expected error for build-2")
	}

	err = policy.OnComplete(&builds[0])
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// No type-check as this error is returned as kerrors.aggregate
	if err := policy.OnComplete(&builds[1]); err == nil {
		t.Errorf("expected error for build-2")
	}
}
