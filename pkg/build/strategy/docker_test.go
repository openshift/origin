package strategy

import (
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/build/api"
)

func TestDockerCreateBuildPod(t *testing.T) {
	const dockerRegistry = "docker-test-registry"
	strategy := NewDockerBuildStrategy("docker-test-image", false)
	expected := mockDockerBuild()
	actual := strategy.CreateBuildPod(expected, dockerRegistry)

	if actual.JSONBase.ID != expected.PodID {
		t.Errorf("Expected %s, but got %s!", expected.PodID, actual.JSONBase.ID)
	}
	if actual.DesiredState.Manifest.Version != "v1beta1" {
		t.Error("Expected v1beta1, but got %s!, actual.DesiredState.Manifest.Version")
	}
	container := actual.DesiredState.Manifest.Containers[0]
	if container.Name != "docker-build" {
		t.Errorf("Expected docker-build, but got %s!", container.Name)
	}
	if container.Image != strategy.dockerBuilderImage {
		t.Errorf("Expected %s image, got %s!", container.Image,
			strategy.dockerBuilderImage)
	}
	if container.RestartPolicy != "runOnce" {
		t.Errorf("Expected runOnce, but got %s!", container.RestartPolicy)
	}
	if e := container.Env[0]; e.Name != "BUILD_TAG" && e.Value != expected.Input.ImageTag {
		t.Errorf("Expected %s, got %s:%s!", expected.Input.ImageTag, e.Name, e.Value)
	}
	if e := container.Env[1]; e.Name != "DOCKER_CONTEXT_URL" && e.Value != expected.Input.SourceURI {
		t.Errorf("Expected %s, got %s:%s!", expected.Input.ImageTag, e.Name, e.Value)
	}
	if e := container.Env[2]; e.Name != "DOCKER_REGISTRY" && e.Value != dockerRegistry {
		t.Errorf("Expected %s got %s:%s!", dockerRegistry, e.Name, e.Value)
	}
}

func mockDockerBuild() *api.Build {
	return &api.Build{
		JSONBase: kubeapi.JSONBase{
			ID: "dockerBuild",
		},
		Input: api.BuildInput{
			Type:      api.DockerBuildType,
			SourceURI: "http://my.build.com/the/dockerbuild/Dockerfile",
			ImageTag:  "repository/dockerBuild",
		},
		Status: api.BuildNew,
		PodID:  "-the-pod-id",
		Labels: map[string]string{
			"name": "dockerBuild",
		},
	}
}
