package prune

import (
	"sort"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util/sets"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

type mockPruneRecorder struct {
	set sets.String
	err error
}

func (m *mockPruneRecorder) Handler(build *buildapi.Build) error {
	m.set.Insert(build.Name)
	return m.err
}

func (m *mockPruneRecorder) Verify(t *testing.T, expected sets.String) {
	if len(m.set) != len(expected) || !m.set.HasAll(expected.List()...) {
		expectedValues := expected.List()
		actualValues := m.set.List()
		sort.Strings(expectedValues)
		sort.Strings(actualValues)
		t.Errorf("expected \n\t%v\n, actual \n\t%v\n", expectedValues, actualValues)
	}
}

func TestPruneTask(t *testing.T) {
	BuildPhaseOptions := []buildapi.BuildPhase{
		buildapi.BuildPhaseCancelled,
		buildapi.BuildPhaseComplete,
		buildapi.BuildPhaseError,
		buildapi.BuildPhaseFailed,
		buildapi.BuildPhaseNew,
		buildapi.BuildPhasePending,
		buildapi.BuildPhaseRunning,
	}
	BuildPhaseFilter := []buildapi.BuildPhase{
		buildapi.BuildPhaseCancelled,
		buildapi.BuildPhaseComplete,
		buildapi.BuildPhaseError,
		buildapi.BuildPhaseFailed,
	}
	BuildPhaseFilterSet := sets.String{}
	for _, BuildPhase := range BuildPhaseFilter {
		BuildPhaseFilterSet.Insert(string(BuildPhase))
	}

	for _, orphans := range []bool{true, false} {
		for _, BuildPhaseOption := range BuildPhaseOptions {
			keepYoungerThan := time.Hour

			now := unversioned.Now()
			old := unversioned.NewTime(now.Time.Add(-1 * keepYoungerThan))

			buildConfigs := []*buildapi.BuildConfig{}
			builds := []*buildapi.Build{}

			buildConfig := mockBuildConfig("a", "build-config")
			buildConfigs = append(buildConfigs, buildConfig)

			builds = append(builds, withCreated(withStatus(mockBuild("a", "build-1", buildConfig), BuildPhaseOption), now))
			builds = append(builds, withCreated(withStatus(mockBuild("a", "build-2", buildConfig), BuildPhaseOption), old))
			builds = append(builds, withCreated(withStatus(mockBuild("a", "orphan-build-1", nil), BuildPhaseOption), now))
			builds = append(builds, withCreated(withStatus(mockBuild("a", "orphan-build-2", nil), BuildPhaseOption), old))

			keepComplete := 1
			keepFailed := 1
			expectedValues := sets.String{}
			filter := &andFilter{
				filterPredicates: []FilterPredicate{NewFilterBeforePredicate(keepYoungerThan)},
			}
			dataSet := NewDataSet(buildConfigs, filter.Filter(builds))
			resolver := NewPerBuildConfigResolver(dataSet, keepComplete, keepFailed)
			if orphans {
				resolver = &mergeResolver{
					resolvers: []Resolver{resolver, NewOrphanBuildResolver(dataSet, BuildPhaseFilter)},
				}
			}
			expectedBuilds, err := resolver.Resolve()
			for _, build := range expectedBuilds {
				expectedValues.Insert(build.Name)
			}

			recorder := &mockPruneRecorder{set: sets.String{}}
			task := NewPruneTasker(buildConfigs, builds, keepYoungerThan, orphans, keepComplete, keepFailed, recorder.Handler)
			err = task.PruneTask()
			if err != nil {
				t.Errorf("Unexpected error %v", err)
			}
			recorder.Verify(t, expectedValues)
		}
	}

}
