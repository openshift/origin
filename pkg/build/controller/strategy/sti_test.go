package strategy

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/api/v1beta1"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

type FakeTempDirCreator struct{}

func (t *FakeTempDirCreator) CreateTempDirectory() (string, error) {
	return "test_temp", nil
}

func TestSTICreateBuildPod(t *testing.T) {
	strategy := &STIBuildStrategy{
		Image:                "sti-test-image",
		TempDirectoryCreator: &FakeTempDirCreator{},
		UseLocalImages:       true,
		Codec:                v1beta1.Codec,
	}

	expected := mockSTIBuild()
	actual, _ := strategy.CreateBuildPod(expected)

	if actual.ObjectMeta.Name != expected.PodName {
		t.Errorf("Expected %s, but got %s!", expected.PodName, actual.ObjectMeta.Name)
	}
	container := actual.Spec.Containers[0]
	if container.Name != "sti-build" {
		t.Errorf("Expected sti-build, but got %s!", container.Name)
	}
	if container.Image != strategy.Image {
		t.Errorf("Expected %s image, got %s!", container.Image, strategy.Image)
	}
	if container.ImagePullPolicy != kapi.PullIfNotPresent {
		t.Errorf("Expected %v, got %v", kapi.PullIfNotPresent, container.ImagePullPolicy)
	}
	if actual.Spec.RestartPolicy.Never == nil {
		t.Errorf("Expected never, got %#v", actual.Spec.RestartPolicy)
	}
	if len(container.Env) != 3 {
		t.Fatalf("Expected 3 elements in Env table, got %d", len(container.Env))
	}
	found := false
	for _, v := range container.Env {
		if v.Name == "FOO" && v.Value == "bar" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Expected variable FOO be defined for the container")
	}
	buildJSON, _ := v1beta1.Codec.Encode(expected)
	errorCases := map[int][]string{
		0: {"BUILD", string(buildJSON)},
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
				Type: buildapi.STIBuildStrategyType,
				STIStrategy: &buildapi.STIBuildStrategy{
					Image:   "repository/sti-builder",
					Scripts: "http://my.build.com/the/sti/scripts",
					Env: []kapi.EnvVar{
						{Name: "FOO", Value: "bar"},
					},
				},
			},
			Output: buildapi.BuildOutput{
				DockerImageReference: "docker-registry/repository/stiBuild",
			},
		},
		Status:  buildapi.BuildStatusNew,
		PodName: "-the-pod-id",
	}
}
