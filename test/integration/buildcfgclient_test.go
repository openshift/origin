// +build integration,!no-etcd

package integration

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

func init() {
	requireEtcd()
}

func TestListBuildConfigs(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()

	buildConfigs, err := openshift.Client.BuildConfigs(testNamespace).List(labels.Everything(), labels.Everything())
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if len(buildConfigs.Items) != 0 {
		t.Errorf("Expected no buildConfigs, got %#v", buildConfigs.Items)
	}
}

func TestCreateBuildConfig(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()
	buildConfig := mockBuildConfig()

	expected, err := openshift.Client.BuildConfigs(testNamespace).Create(buildConfig)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if expected.Name == "" {
		t.Errorf("Unexpected empty buildConfig ID %v", expected)
	}

	buildConfigs, err := openshift.Client.BuildConfigs(testNamespace).List(labels.Everything(), labels.Everything())
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if len(buildConfigs.Items) != 1 {
		t.Errorf("Expected one buildConfig, got %#v", buildConfigs.Items)
	}
}

func TestUpdateBuildConfig(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()
	buildConfig := mockBuildConfig()

	actual, err := openshift.Client.BuildConfigs(testNamespace).Create(buildConfig)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	actual, err = openshift.Client.BuildConfigs(testNamespace).Get(actual.Name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if _, err := openshift.Client.BuildConfigs(testNamespace).Update(actual); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestDeleteBuildConfig(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()
	buildConfig := mockBuildConfig()

	actual, err := openshift.Client.BuildConfigs(testNamespace).Create(buildConfig)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := openshift.Client.BuildConfigs(testNamespace).Delete(actual.Name); err != nil {
		t.Fatalf("Unxpected error: %v", err)
	}
}

func TestWatchBuildConfigs(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()
	buildConfig := mockBuildConfig()

	watch, err := openshift.Client.BuildConfigs(testNamespace).Watch(labels.Everything(), labels.Everything(), "0")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer watch.Stop()

	expected, err := openshift.Client.BuildConfigs(testNamespace).Create(buildConfig)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	event := <-watch.ResultChan()
	actual := event.Object.(*buildapi.BuildConfig)

	if e, a := expected.Name, actual.Name; e != a {
		t.Errorf("Expected buildConfig Name %s, got %s", e, a)
	}
}

func mockBuildConfig() *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Labels: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},
		},
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				Type: buildapi.GithubWebHookBuildTriggerType,
				GithubWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
		},
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://my.docker/build",
				},
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{
					ContextDir: "context",
				},
			},
			Output: buildapi.BuildOutput{
				DockerImageReference: "namespace/builtimage",
			},
		},
	}
}

func TestBuildConfigClient(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()

	buildConfigs, err := openshift.Client.BuildConfigs(testNamespace).List(labels.Everything(), labels.Everything())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(buildConfigs.Items) != 0 {
		t.Errorf("expected no buildConfigs, got %#v", buildConfigs)
	}

	// get a validation error
	buildConfig := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Labels: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},
		},
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://my.docker/build",
				},
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{
					ContextDir: "context",
				},
			},
			Output: buildapi.BuildOutput{
				DockerImageReference: "namespace/builtimage",
			},
		},
	}

	// get a created buildConfig
	got, err := openshift.Client.BuildConfigs(testNamespace).Create(buildConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name == "" {
		t.Errorf("unexpected empty buildConfig ID %v", got)
	}

	// get a list of buildConfigs
	buildConfigs, err = openshift.Client.BuildConfigs(testNamespace).List(labels.Everything(), labels.Everything())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(buildConfigs.Items) != 1 {
		t.Errorf("expected one buildConfig, got %#v", buildConfigs)
	}
	actual := buildConfigs.Items[0]
	if actual.Name != got.Name {
		t.Errorf("expected buildConfig %#v, got %#v", got, actual)
	}

	// delete a buildConfig
	err = openshift.Client.BuildConfigs(testNamespace).Delete(got.Name)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	buildConfigs, err = openshift.Client.BuildConfigs(testNamespace).List(labels.Everything(), labels.Everything())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(buildConfigs.Items) != 0 {
		t.Errorf("expected no buildConfigs, got %#v", buildConfigs)
	}
}
