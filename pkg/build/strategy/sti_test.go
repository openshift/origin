package strategy

import (
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/build/api"
)

func TestSTICreateBuildPod(t *testing.T) {
	const dockerRegistry = "sti-test-registry"
	strategy := NewSTIBuildStrategy("sti-test-image", false)
	expected := mockSTIBuild()
	actual := strategy.CreateBuildPod(expected, dockerRegistry)

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
		t.Errorf("Expected %s image, got %s!", container.Image,
			strategy.stiBuilderImage)
	}
	if container.RestartPolicy != "runOnce" {
		t.Errorf("Expected runOnce, but got %s!", container.RestartPolicy)
	}
	if e := container.Env[0]; e.Name != "BUILD_TAG" && e.Value != expected.Input.ImageTag {
		t.Errorf("Expected %s, got %s:%s!", expected.Input.ImageTag, e.Name, e.Value)
	}
	if e := container.Env[1]; e.Name != "DOCKER_REGISTRY" && e.Value != dockerRegistry {
		t.Errorf("Expected %s got %s:%s!", dockerRegistry, e.Name, e.Value)
	}
	if e := container.Env[2]; e.Name != "SOURCE_REF" && e.Value != expected.Input.SourceURI {
		t.Errorf("Expected %s got %s:%s!", expected.Input.SourceURI, e.Name, e.Value)
	}
	if e := container.Env[3]; e.Name != "BUILDER_IMAGE" && e.Value != expected.Input.BuilderImage {
		t.Errorf("Expected %s, got %s:%s!", expected.Input.BuilderImage, e.Name, e.Value)
	}
}

func mockSTIBuild() *api.Build {
	return &api.Build{
		JSONBase: kubeapi.JSONBase{
			ID: "stiBuild",
		},
		Input: api.BuildInput{
			Type:      api.STIBuildType,
			SourceURI: "http://my.build.com/the/stibuild/Dockerfile",
			ImageTag:  "repository/stiBuild",
		},
		Status: api.BuildNew,
		PodID:  "-the-pod-id",
		Labels: map[string]string{
			"name": "stiBuild",
		},
	}
}
