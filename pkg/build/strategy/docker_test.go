package strategy

import (
	"encoding/json"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/build/api"
)

func TestDockerCreateBuildPod(t *testing.T) {
	strategy := NewDockerBuildStrategy("docker-test-image", true)
	expected := mockDockerBuild()
	actual, _ := strategy.CreateBuildPod(expected)

	if actual.TypeMeta.ID != expected.PodID {
		t.Errorf("Expected %s, but got %s!", expected.PodID, actual.TypeMeta.ID)
	}
	if actual.DesiredState.Manifest.Version != "v1beta1" {
		t.Error("Expected v1beta1, but got %s!, actual.DesiredState.Manifest.Version")
	}
	container := actual.DesiredState.Manifest.Containers[0]
	if container.Name != "docker-build" {
		t.Errorf("Expected docker-build, but got %s!", container.Name)
	}
	if container.Image != strategy.dockerBuilderImage {
		t.Errorf("Expected %s image, got %s!", container.Image, strategy.dockerBuilderImage)
	}
	if container.ImagePullPolicy != kapi.PullIfNotPresent {
		t.Errorf("Expected %v, got %v", kapi.PullIfNotPresent, container.ImagePullPolicy)
	}
	if actual.DesiredState.Manifest.RestartPolicy.Never == nil {
		t.Errorf("Expected never, got %#v", actual.DesiredState.Manifest.RestartPolicy)
	}
	if len(container.Env) != 7 {
		t.Fatalf("Expected 7 elements in Env table, got %d", len(container.Env))
	}
	buildJson, _ := json.Marshal(expected)
	errorCases := map[int][]string{
		0: {"BUILD_TAG", expected.Input.ImageTag},
		1: {"SOURCE_URI", expected.Source.Git.URI},
		2: {"SOURCE_REF", expected.Source.Git.Ref},
		3: {"SOURCE_ID", expected.Revision.Git.Commit},
		4: {"REGISTRY", expected.Input.Registry},
		5: {"CONTEXT_DIR", expected.Input.DockerInput.ContextDir},
		6: {"BUILD", string(buildJson)},
	}
	for index, exp := range errorCases {
		if e := container.Env[index]; e.Name != exp[0] || e.Value != exp[1] {
			t.Errorf("Expected %s:%s, got %s:%s!\n", exp[0], exp[1], e.Name, e.Value)
		}
	}
}

func mockDockerBuild() *api.Build {
	return &api.Build{
		TypeMeta: kapi.TypeMeta{
			ID: "dockerBuild",
		},
		Revision: api.SourceRevision{
			Git: &api.GitSourceRevision{},
		},
		Source: api.BuildSource{
			Git: &api.GitBuildSource{
				URI: "http://my.build.com/the/dockerbuild/Dockerfile",
			},
		},
		Input: api.BuildInput{
			ImageTag:    "repository/dockerBuild",
			Registry:    "docker-registry",
			DockerInput: &api.DockerBuildInput{ContextDir: "my/test/dir"},
		},
		Status: api.BuildNew,
		PodID:  "-the-pod-id",
		Labels: map[string]string{
			"name": "dockerBuild",
		},
	}
}
