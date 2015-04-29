package strategy

import (
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/resource"

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
	expectedLabels := make(map[string]string)
	for k, v := range expected.Labels {
		expectedLabels[k] = v
	}
	expectedLabels[buildapi.BuildLabel] = expected.Name
	if !reflect.DeepEqual(expectedLabels, actual.Labels) {
		t.Errorf("Pod Labels does not match Build Labels!")
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
	if !kapi.Semantic.DeepEqual(container.Resources, expected.Parameters.Resources) {
		t.Fatalf("Expected actual=expected, %v != %v", container.Resources, expected.Parameters.Resources)
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
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "repository/sti-builder",
					},
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
			Resources: kapi.ResourceRequirements{
				Limits: kapi.ResourceList{
					kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
					kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("10G"),
				},
			},
		},
		Status: buildapi.BuildStatusNew,
	}
}
