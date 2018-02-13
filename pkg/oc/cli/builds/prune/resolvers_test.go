package prune

import (
	"fmt"
	"sort"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

type mockResolver struct {
	builds []*buildapi.Build
	err    error
}

func (m *mockResolver) Resolve() ([]*buildapi.Build, error) {
	return m.builds, m.err
}

func TestMergeResolver(t *testing.T) {
	resolverA := &mockResolver{
		builds: []*buildapi.Build{
			mockBuild("a", "b", nil),
		},
	}
	resolverB := &mockResolver{
		builds: []*buildapi.Build{
			mockBuild("c", "d", nil),
		},
	}
	resolver := &mergeResolver{resolvers: []Resolver{resolverA, resolverB}}
	results, err := resolver.Resolve()
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Unexpected results %v", results)
	}
	expectedNames := sets.NewString("b", "d")
	for _, build := range results {
		if !expectedNames.Has(build.Name) {
			t.Errorf("Unexpected name %v", build.Name)
		}
	}
}

func TestOrphanBuildResolver(t *testing.T) {
	activeBuildConfig := mockBuildConfig("a", "active-build-config")
	inactiveBuildConfig := mockBuildConfig("a", "inactive-build-config")

	buildConfigs := []*buildapi.BuildConfig{activeBuildConfig}
	builds := []*buildapi.Build{}

	expectedNames := sets.String{}
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

	for _, BuildPhaseOption := range BuildPhaseOptions {
		builds = append(builds, withStatus(mockBuild("a", string(BuildPhaseOption)+"-active", activeBuildConfig), BuildPhaseOption))
		builds = append(builds, withStatus(mockBuild("a", string(BuildPhaseOption)+"-inactive", inactiveBuildConfig), BuildPhaseOption))
		builds = append(builds, withStatus(mockBuild("a", string(BuildPhaseOption)+"-orphan", nil), BuildPhaseOption))
		if BuildPhaseFilterSet.Has(string(BuildPhaseOption)) {
			expectedNames.Insert(string(BuildPhaseOption) + "-inactive")
			expectedNames.Insert(string(BuildPhaseOption) + "-orphan")
		}
	}

	dataSet := NewDataSet(buildConfigs, builds)
	resolver := NewOrphanBuildResolver(dataSet, BuildPhaseFilter)
	results, err := resolver.Resolve()
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	foundNames := sets.String{}
	for _, result := range results {
		foundNames.Insert(result.Name)
	}
	if len(foundNames) != len(expectedNames) || !expectedNames.HasAll(foundNames.List()...) {
		t.Errorf("expected %v, actual %v", expectedNames, foundNames)
	}
}

func TestPerBuildConfigResolver(t *testing.T) {
	BuildPhaseOptions := []buildapi.BuildPhase{
		buildapi.BuildPhaseCancelled,
		buildapi.BuildPhaseComplete,
		buildapi.BuildPhaseError,
		buildapi.BuildPhaseFailed,
		buildapi.BuildPhaseNew,
		buildapi.BuildPhasePending,
		buildapi.BuildPhaseRunning,
	}
	buildConfigs := []*buildapi.BuildConfig{
		mockBuildConfig("a", "build-config-1"),
		mockBuildConfig("b", "build-config-2"),
	}
	buildsPerStatus := 100
	builds := []*buildapi.Build{}
	for _, buildConfig := range buildConfigs {
		for _, BuildPhaseOption := range BuildPhaseOptions {
			for i := 0; i < buildsPerStatus; i++ {
				build := withStatus(mockBuild(buildConfig.Namespace, fmt.Sprintf("%v-%v-%v", buildConfig.Name, BuildPhaseOption, i), buildConfig), BuildPhaseOption)
				builds = append(builds, build)
			}
		}
	}

	now := metav1.Now()
	for i := range builds {
		creationTimestamp := metav1.NewTime(now.Time.Add(-1 * time.Duration(i) * time.Hour))
		builds[i].CreationTimestamp = creationTimestamp
	}

	// test number to keep at varying ranges
	for keep := 0; keep < buildsPerStatus*2; keep++ {
		dataSet := NewDataSet(buildConfigs, builds)

		expectedNames := sets.String{}
		buildCompleteStatusFilterSet := sets.NewString(string(buildapi.BuildPhaseComplete))
		buildFailedStatusFilterSet := sets.NewString(string(buildapi.BuildPhaseCancelled), string(buildapi.BuildPhaseError), string(buildapi.BuildPhaseFailed))

		for _, buildConfig := range buildConfigs {
			buildItems, err := dataSet.ListBuildsByBuildConfig(buildConfig)
			if err != nil {
				t.Errorf("Unexpected err %v", err)
			}
			var completeBuilds, failedBuilds []*buildapi.Build
			for _, build := range buildItems {
				if buildCompleteStatusFilterSet.Has(string(build.Status.Phase)) {
					completeBuilds = append(completeBuilds, build)
				} else if buildFailedStatusFilterSet.Has(string(build.Status.Phase)) {
					failedBuilds = append(failedBuilds, build)
				}
			}
			sort.Sort(sort.Reverse(buildapi.BuildPtrSliceByCreationTimestamp(completeBuilds)))
			sort.Sort(sort.Reverse(buildapi.BuildPtrSliceByCreationTimestamp(failedBuilds)))
			var purgeComplete, purgeFailed []*buildapi.Build
			if keep >= 0 && keep < len(completeBuilds) {
				purgeComplete = completeBuilds[keep:]
			}
			if keep >= 0 && keep < len(failedBuilds) {
				purgeFailed = failedBuilds[keep:]
			}
			for _, build := range purgeComplete {
				expectedNames.Insert(build.Name)
			}
			for _, build := range purgeFailed {
				expectedNames.Insert(build.Name)
			}
		}

		resolver := NewPerBuildConfigResolver(dataSet, keep, keep)
		results, err := resolver.Resolve()
		if err != nil {
			t.Errorf("Unexpected error %v", err)
		}
		foundNames := sets.String{}
		for _, result := range results {
			foundNames.Insert(result.Name)
		}
		if len(foundNames) != len(expectedNames) || !expectedNames.HasAll(foundNames.List()...) {
			expectedValues := expectedNames.List()
			actualValues := foundNames.List()
			sort.Strings(expectedValues)
			sort.Strings(actualValues)
			t.Errorf("keep %v\n, expected \n\t%v\n, actual \n\t%v\n", keep, expectedValues, actualValues)
		}
	}
}
