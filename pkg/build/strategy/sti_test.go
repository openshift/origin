package strategy

import (
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/build/api"
)

type FakeTempDirCreator struct{}

func (t *FakeTempDirCreator) CreateTempDirectory() (string, error) {
	return "test_temp", nil
}

func TestSTICreateBuildPod(t *testing.T) {
	strategy := NewSTIBuildStrategy("sti-test-image", &FakeTempDirCreator{}, true)
	expected := mockSTIBuild()
	actual, _ := strategy.CreateBuildPod(expected)

	if actual.JSONBase.ID != expected.PodID {
		t.Errorf("Expected %s, but got %s!", expected.PodID, actual.JSONBase.ID)
	}
	if actual.DesiredState.Manifest.Version != "v1beta1" {
		t.Error("Expected v1beta1, but got %s!, actual.DesiredState.Manifest.Version")
	}
	container := actual.DesiredState.Manifest.Containers[0]
	if container.Name != "sti-build" {
		t.Errorf("Expected sti-build, but got %s!", container.Name)
	}
	if container.Image != strategy.stiBuilderImage {
		t.Errorf("Expected %s image, got %s!", container.Image, strategy.stiBuilderImage)
	}
	if container.ImagePullPolicy != kubeapi.PullIfNotPresent {
		t.Errorf("Expected %v, got %v", kubeapi.PullIfNotPresent, container.ImagePullPolicy)
	}
	if actual.DesiredState.Manifest.RestartPolicy.Never == nil {
		t.Errorf("Expected never, got %#v", actual.DesiredState.Manifest.RestartPolicy)
	}
	if len(container.Env) != 6 {
		t.Fatalf("Expected 6 elements in Env table, got %d", len(container.Env))
	}
	errorCases := map[int][]string{
		0: {"BUILD_TAG", expected.Input.ImageTag},
		1: {"SOURCE_URI", expected.Input.SourceURI},
		2: {"SOURCE_REF", expected.Input.SourceRef},
		3: {"REGISTRY", expected.Input.Registry},
		4: {"BUILDER_IMAGE", expected.Input.STIInput.BuilderImage},
		5: {"TEMP_DIR", "test_temp"},
	}
	for index, exp := range errorCases {
		if e := container.Env[index]; e.Name != exp[0] || e.Value != exp[1] {
			t.Errorf("Expected %s:%s, got %s:%s!\n", exp[0], exp[1], e.Name, e.Value)
		}
	}
}

func mockSTIBuild() *api.Build {
	return &api.Build{
		JSONBase: kubeapi.JSONBase{
			ID: "stiBuild",
		},
		Input: api.BuildInput{
			SourceURI: "http://my.build.com/the/stibuild/Dockerfile",
			ImageTag:  "repository/stiBuild",
			Registry:  "docker-registry",
			STIInput:  &api.STIBuildInput{BuilderImage: "repository/sti-builder"},
		},
		Status: api.BuildNew,
		PodID:  "-the-pod-id",
		Labels: map[string]string{
			"name": "stiBuild",
		},
	}
}
