package strategy

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/api/v1beta1"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

func TestDockerCreateBuildPod(t *testing.T) {
	strategy := DockerBuildStrategy{
		Image: "docker-test-image",
		Codec: v1beta1.Codec,
	}

	expected := mockDockerBuild()
	actual, err := strategy.CreateBuildPod(expected)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if expected, actual := buildutil.GetBuildPodName(expected), actual.ObjectMeta.Name; expected != actual {
		t.Errorf("Expected %s, but got %s!", expected, actual)
	}
	container := actual.Spec.Containers[0]
	if container.Name != "docker-build" {
		t.Errorf("Expected docker-build, but got %s!", container.Name)
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
	if len(container.Env) != 3 {
		t.Fatalf("Expected 3 elements in Env table, got %d", len(container.Env))
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

func mockDockerBuild() *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "dockerBuild",
			Labels: map[string]string{
				"name": "dockerBuild",
			},
		},
		Parameters: buildapi.BuildParameters{
			Revision: &buildapi.SourceRevision{
				Git: &buildapi.GitSourceRevision{},
			},
			Source: buildapi.BuildSource{
				Git: &buildapi.GitBuildSource{
					URI: "http://my.build.com/the/dockerbuild/Dockerfile",
				},
				ContextDir: "my/test/dir",
			},
			Strategy: buildapi.BuildStrategy{
				Type:           buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{},
			},
			Output: buildapi.BuildOutput{
				DockerImageReference: "docker-registry/repository/dockerBuild",
				PushSecretName:       "foo",
			},
		},
		Status: buildapi.BuildStatusNew,
	}
}
