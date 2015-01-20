package strategy

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/api/v1beta1"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

func TestCustomCreateBuildPod(t *testing.T) {
	strategy := CustomBuildStrategy{
		UseLocalImages: true,
	}

	expectedBad := mockCustomBuild()
	expectedBad.Parameters.Strategy.CustomStrategy.Image = ""
	if _, err := strategy.CreateBuildPod(expectedBad); err == nil {
		t.Errorf("Expected error when Image is empty, got nothing")
	}

	expected := mockCustomBuild()
	actual, err := strategy.CreateBuildPod(expected)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if actual.ObjectMeta.Name != expected.PodName {
		t.Errorf("Expected %s, but got %s!", expected.PodName, actual.ObjectMeta.Name)
	}

	container := actual.Spec.Containers[0]
	if container.Name != "custom-build" {
		t.Errorf("Expected custom-build, but got %s!", container.Name)
	}
	if container.ImagePullPolicy != kapi.PullIfNotPresent {
		t.Errorf("Expected %v, got %v", kapi.PullIfNotPresent, container.ImagePullPolicy)
	}
	if actual.Spec.RestartPolicy.Never == nil {
		t.Errorf("Expected never, got %#v", actual.Spec.RestartPolicy)
	}
	buildJSON, _ := v1beta1.Codec.Encode(expected)
	errorCases := map[int][]string{
		0: {"BUILD", string(buildJSON)},
	}
	standardEnv := []string{"SOURCE_URI", "SOURCE_REF", "OUTPUT_IMAGE", "OUTPUT_REGISTRY"}
	for index, exp := range errorCases {
		if e := container.Env[index]; e.Name != exp[0] || e.Value != exp[1] {
			t.Errorf("Expected %s:%s, got %s:%s!\n", exp[0], exp[1], e.Name, e.Value)
		}
	}
	for _, name := range standardEnv {
		found := false
		for _, item := range container.Env {
			if (item.Name == name) && len(item.Value) != 0 {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected %s variable to be set", name)
		}
	}
}

func mockCustomBuild() *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "customBuild",
			Labels: map[string]string{
				"name": "customBuild",
			},
		},
		Parameters: buildapi.BuildParameters{
			Revision: &buildapi.SourceRevision{
				Git: &buildapi.GitSourceRevision{},
			},
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://my.build.com/the/dockerbuild/Dockerfile",
					Ref: "master",
				},
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.CustomBuildStrategyType,
				CustomStrategy: &buildapi.CustomBuildStrategy{
					Image: "builder-image",
					Env: []kapi.EnvVar{
						{"FOO", "BAR"},
					},
					ExposeDockerSocket: true,
				},
			},
			Output: buildapi.BuildOutput{
				DockerImageReference: "docker-registry/repository/customBuild",
			},
		},
		Status:  buildapi.BuildStatusNew,
		PodName: "-the-pod-id",
	}
}
