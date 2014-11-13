package strategy

import (
	"encoding/json"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

type FakeTempDirCreator struct{}

func (t *FakeTempDirCreator) CreateTempDirectory() (string, error) {
	return "test_temp", nil
}

func TestSTICreateBuildPod(t *testing.T) {
	strategy := &STIBuildStrategy{
		BuilderImage:         "sti-test-image",
		TempDirectoryCreator: &FakeTempDirCreator{},
		UseLocalImages:       true,
	}

	expected := mockSTIBuild()
	actual, _ := strategy.CreateBuildPod(expected)

	if actual.ObjectMeta.Name != expected.PodName {
		t.Errorf("Expected %s, but got %s!", expected.PodName, actual.ObjectMeta.Name)
	}
	if actual.DesiredState.Manifest.Version != "v1beta1" {
		t.Error("Expected v1beta1, but got %s!, actual.DesiredState.Manifest.Version")
	}
	container := actual.DesiredState.Manifest.Containers[0]
	if container.Name != "sti-build" {
		t.Errorf("Expected sti-build, but got %s!", container.Name)
	}
	if container.Image != strategy.BuilderImage {
		t.Errorf("Expected %s image, got %s!", container.Image, strategy.BuilderImage)
	}
	if container.ImagePullPolicy != kapi.PullIfNotPresent {
		t.Errorf("Expected %v, got %v", kapi.PullIfNotPresent, container.ImagePullPolicy)
	}
	if actual.DesiredState.Manifest.RestartPolicy.Never == nil {
		t.Errorf("Expected never, got %#v", actual.DesiredState.Manifest.RestartPolicy)
	}
	if len(container.Env) != 1 {
		t.Fatalf("Expected 1 elements in Env table, got %d", len(container.Env))
	}
	buildJson, _ := json.Marshal(expected)
	errorCases := map[int][]string{
		0: {"BUILD", string(buildJson)},
	}
	for index, exp := range errorCases {
		if e := container.Env[index]; e.Name != exp[0] || e.Value != exp[1] {
			t.Errorf("Expected %s:%s, got %s:%s!\n", exp[0], exp[1], e.Name, e.Value)
		}
	}
}

func mockSTIBuild() *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "stiBuild",
			Labels: map[string]string{
				"name": "stiBuild",
			},
		},
		Parameters: buildapi.BuildParameters{
			Revision: &buildapi.SourceRevision{
				Git: &buildapi.GitSourceRevision{},
			},
			Source: buildapi.BuildSource{
				Git: &buildapi.GitBuildSource{
					URI: "http://my.build.com/the/stibuild/Dockerfile",
				},
			},
			Strategy: buildapi.BuildStrategy{
				Type:        buildapi.STIBuildStrategyType,
				STIStrategy: &buildapi.STIBuildStrategy{BuilderImage: "repository/sti-builder"},
			},
			Output: buildapi.BuildOutput{
				ImageTag: "repository/stiBuild",
				Registry: "docker-registry",
			},
		},
		Status:  buildapi.BuildStatusNew,
		PodName: "-the-pod-id",
	}
}
