package builds

import (
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	buildv1 "github.com/openshift/api/build/v1"
)

func mockBuildConfig(namespace, name string) *buildv1.BuildConfig {
	return &buildv1.BuildConfig{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
}

func withCreated(build *buildv1.Build, creationTimestamp metav1.Time) *buildv1.Build {
	build.CreationTimestamp = creationTimestamp
	return build
}

func withStatus(build *buildv1.Build, status buildv1.BuildPhase) *buildv1.Build {
	build.Status.Phase = status
	return build
}

func mockBuild(namespace, name string, buildConfig *buildv1.BuildConfig) *buildv1.Build {
	build := &buildv1.Build{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
	if buildConfig != nil {
		build.Status.Config = &corev1.ObjectReference{
			Name:      buildConfig.Name,
			Namespace: buildConfig.Namespace,
		}
	}
	build.Status.Phase = buildv1.BuildPhaseNew
	return build
}

func TestBuildByBuildConfigIndexFunc(t *testing.T) {
	buildWithConfig := &buildv1.Build{
		Status: buildv1.BuildStatus{
			Config: &corev1.ObjectReference{
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
	buildWithNoConfig := &buildv1.Build{}
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
	now := metav1.Now()
	old := metav1.NewTime(now.Time.Add(-1 * youngerThan))
	builds := []*buildv1.Build{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "old",
				CreationTimestamp: old,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
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
	builds := []*buildv1.Build{}
	buildConfigs := []*buildv1.BuildConfig{}
	dataSet := NewDataSet(buildConfigs, builds)
	_, exists, err := dataSet.GetBuildConfig(&buildv1.Build{})
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
	buildResults, err = dataSet.ListBuildsByBuildConfig(&buildv1.BuildConfig{})
	if err != nil {
		t.Errorf("Unexpected result %v", err)
	}
	if len(buildResults) != 0 {
		t.Errorf("Unexpected result %v", buildResults)
	}
}

func TestPopuldatedDataSet(t *testing.T) {
	buildConfigs := []*buildv1.BuildConfig{
		mockBuildConfig("a", "build-config-1"),
		mockBuildConfig("b", "build-config-2"),
	}
	builds := []*buildv1.Build{
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
