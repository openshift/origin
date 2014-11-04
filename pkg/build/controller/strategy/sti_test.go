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

	if actual.TypeMeta.ID != expected.PodID {
		t.Errorf("Expected %s, but got %s!", expected.PodID, actual.TypeMeta.ID)
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
	if len(container.Env) != 8 {
		t.Fatalf("Expected 8 elements in Env table, got %d", len(container.Env))
	}
	buildJson, _ := json.Marshal(expected)
	errorCases := map[int][]string{
		0: {"SOURCE_URI", expected.Parameters.Source.Git.URI},
		1: {"SOURCE_REF", expected.Parameters.Source.Git.Ref},
		2: {"SOURCE_ID", expected.Parameters.Revision.Git.Commit},
		3: {"BUILDER_IMAGE", expected.Parameters.Strategy.STIStrategy.BuilderImage},
		4: {"BUILD_TAG", expected.Parameters.Output.ImageTag},
		5: {"REGISTRY", expected.Parameters.Output.Registry},
		6: {"BUILD", string(buildJson)},
		7: {"TEMP_DIR", "test_temp"},
	}
	for index, exp := range errorCases {
		if e := container.Env[index]; e.Name != exp[0] || e.Value != exp[1] {
			t.Errorf("Expected %s:%s, got %s:%s!\n", exp[0], exp[1], e.Name, e.Value)
		}
	}
}

func mockSTIBuild() *buildapi.Build {
	return &buildapi.Build{
		TypeMeta: kapi.TypeMeta{
			ID: "stiBuild",
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
		Status: buildapi.BuildStatusNew,
		PodID:  "-the-pod-id",
		Labels: map[string]string{
			"name": "stiBuild",
		},
	}
}
