package prune

import (
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

func mockBuildConfig(namespace, name string) *buildapi.BuildConfig {
	return &buildapi.BuildConfig{ObjectMeta: kapi.ObjectMeta{Namespace: namespace, Name: name}}
}

func withCreated(build *buildapi.Build, creationTimestamp unversioned.Time) *buildapi.Build {
	build.CreationTimestamp = creationTimestamp
	return build
}

func withStatus(build *buildapi.Build, status buildapi.BuildPhase) *buildapi.Build {
	build.Status.Phase = status
	return build
}

func mockBuild(namespace, name string, buildConfig *buildapi.BuildConfig) *buildapi.Build {
	build := &buildapi.Build{ObjectMeta: kapi.ObjectMeta{Namespace: namespace, Name: name}}
	if buildConfig != nil {
		build.Status.Config = &kapi.ObjectReference{
			Name:      buildConfig.Name,
			Namespace: buildConfig.Namespace,
		}
	}
	build.Status.Phase = buildapi.BuildPhaseNew
	return build
}

func TestBuildByBuildConfigIndexFunc(t *testing.T) {
	buildWithConfig := &buildapi.Build{
		Status: buildapi.BuildStatus{
			Config: &kapi.ObjectReference{
				Name:      "buildConfigName",
				Namespace: "buildConfigNamespace",
			},
		},
	}
	actualKey, err := BuildByBuildConfigIndexFunc(buildWithConfig)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	expectedKey := []string{buildWithConfig.Status.Config.Namespace + "/" + buildWithConfig.Status.Config.Name}
	if !reflect.DeepEqual(actualKey, expectedKey) {
		t.Errorf("expected %#v, actual %#v", expectedKey, actualKey)
	}
	buildWithNoConfig := &buildapi.Build{}
	actualKey, err = BuildByBuildConfigIndexFunc(buildWithNoConfig)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	expectedKey = []string{"orphan"}
	if !reflect.DeepEqual(actualKey, expectedKey) {
		t.Errorf("expected %v, actual %v", expectedKey, actualKey)
	}
}

func TestFilterBeforePredicate(t *testing.T) {
	youngerThan := time.Hour
	now := unversioned.Now()
	old := unversioned.NewTime(now.Time.Add(-1 * youngerThan))
	builds := []*buildapi.Build{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:              "old",
				CreationTimestamp: old,
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:              "new",
				CreationTimestamp: now,
			},
		},
	}
	filter := &andFilter{
		filterPredicates: []FilterPredicate{NewFilterBeforePredicate(youngerThan)},
	}
	result := filter.Filter(builds)
	if len(result) != 1 {
		t.Errorf("Unexpected number of results")
	}
	if expected, actual := "old", result[0].Name; expected != actual {
		t.Errorf("expected %v, actual %v", expected, actual)
	}
}

func TestEmptyDataSet(t *testing.T) {
	builds := []*buildapi.Build{}
	buildConfigs := []*buildapi.BuildConfig{}
	dataSet := NewDataSet(buildConfigs, builds)
	_, exists, err := dataSet.GetBuildConfig(&buildapi.Build{})
	if exists || err != nil {
		t.Errorf("Unexpected result %v, %v", exists, err)
	}
	buildConfigResults, err := dataSet.ListBuildConfigs()
	if err != nil {
		t.Errorf("Unexpected result %v", err)
	}
	if len(buildConfigResults) != 0 {
		t.Errorf("Unexpected result %v", buildConfigResults)
	}
	buildResults, err := dataSet.ListBuilds()
	if err != nil {
		t.Errorf("Unexpected result %v", err)
	}
	if len(buildResults) != 0 {
		t.Errorf("Unexpected result %v", buildResults)
	}
	buildResults, err = dataSet.ListBuildsByBuildConfig(&buildapi.BuildConfig{})
	if err != nil {
		t.Errorf("Unexpected result %v", err)
	}
	if len(buildResults) != 0 {
		t.Errorf("Unexpected result %v", buildResults)
	}
}

func TestPopuldatedDataSet(t *testing.T) {
	buildConfigs := []*buildapi.BuildConfig{
		mockBuildConfig("a", "build-config-1"),
		mockBuildConfig("b", "build-config-2"),
	}
	builds := []*buildapi.Build{
		mockBuild("a", "build-1", buildConfigs[0]),
		mockBuild("a", "build-2", buildConfigs[0]),
		mockBuild("b", "build-3", buildConfigs[1]),
		mockBuild("c", "build-4", nil),
	}
	dataSet := NewDataSet(buildConfigs, builds)
	for _, build := range builds {
		buildConfig, exists, err := dataSet.GetBuildConfig(build)
		if build.Status.Config != nil {
			if err != nil {
				t.Errorf("Item %v, unexpected error: %v", build, err)
			}
			if !exists {
				t.Errorf("Item %v, unexpected result: %v", build, exists)
			}
			if expected, actual := build.Status.Config.Name, buildConfig.Name; expected != actual {
				t.Errorf("expected %v, actual %v", expected, actual)
			}
			if expected, actual := build.Status.Config.Namespace, buildConfig.Namespace; expected != actual {
				t.Errorf("expected %v, actual %v", expected, actual)
			}
		} else {
			if err != nil {
				t.Errorf("Item %v, unexpected error: %v", build, err)
			}
			if exists {
				t.Errorf("Item %v, unexpected result: %v", build, exists)
			}
		}
	}
	expectedNames := sets.NewString("build-1", "build-2")
	buildResults, err := dataSet.ListBuildsByBuildConfig(buildConfigs[0])
	if err != nil {
		t.Errorf("Unexpected result %v", err)
	}
	if len(buildResults) != len(expectedNames) {
		t.Errorf("Unexpected result %v", buildResults)
	}
	for _, build := range buildResults {
		if !expectedNames.Has(build.Name) {
			t.Errorf("Unexpected name: %v", build.Name)
		}
	}
}
