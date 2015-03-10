package strategy

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/api/v1beta1"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

type FakeTempDirCreator struct{}

func (t *FakeTempDirCreator) CreateTempDirectory() (string, error) {
	return "test_temp", nil
}

func TestSTICreateBuildPod(t *testing.T) {
	strategy := &STIBuildStrategy{
		Image:                "sti-test-image",
		TempDirectoryCreator: &FakeTempDirCreator{},
		Codec:                v1beta1.Codec,
	}

	expected := mockSTIBuild()
	actual, err := strategy.CreateBuildPod(expected)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if expected, actual := buildutil.GetBuildPodName(expected), actual.ObjectMeta.Name; expected != actual {
		t.Errorf("Expected %s, but got %s!", expected, actual)
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
	if actual.Spec.RestartPolicy != kapi.RestartPolicyNever {
		t.Errorf("Expected never, got %#v", actual.Spec.RestartPolicy)
	}
	if len(container.Env) != 6 {
		t.Fatalf("Expected 6 elements in Env table, got %d", len(container.Env))
	}
	if len(container.VolumeMounts) != 2 {
		t.Fatalf("Expected 2 volumes in container, got %d", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].MountPath != dockerSocketPath {
		t.Fatalf("Expected %s in first VolumeMount, got %s", dockerSocketPath, container.VolumeMounts[0].MountPath)
	}
	if container.VolumeMounts[1].MountPath != dockerPushSecretMountPath {
		t.Fatalf("Expected %s in first VolumeMount, got %s", dockerPushSecretMountPath, container.VolumeMounts[1].MountPath)
	}
	if len(actual.Spec.Volumes) != 2 {
		t.Fatalf("Expected 2 volumes in Build pod, got %d", len(actual.Spec.Volumes))
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
				PushSecretName:       "foo",
			},
		},
		Status: buildapi.BuildStatusNew,
	}
}
