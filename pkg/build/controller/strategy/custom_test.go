package strategy

import (
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/resource"

	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

func TestCustomCreateBuildPod(t *testing.T) {
	strategy := CustomBuildStrategy{
		Codec: latest.Codec,
	}

	expectedBad := mockCustomBuild(false)
	expectedBad.Spec.Strategy.CustomStrategy.From = kapi.ObjectReference{
		Kind: "DockerImage",
		Name: "",
	}
	if _, err := strategy.CreateBuildPod(expectedBad); err == nil {
		t.Errorf("Expected error when Image is empty, got nothing")
	}

	expected := mockCustomBuild(false)
	actual, err := strategy.CreateBuildPod(expected)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if expected, actual := buildutil.GetBuildPodName(expected), actual.ObjectMeta.Name; expected != actual {
		t.Errorf("Expected %s, but got %s!", expected, actual)
	}
	if !reflect.DeepEqual(map[string]string{buildapi.BuildLabel: expected.Name}, actual.Labels) {
		t.Errorf("Pod Labels does not match Build Labels!")
	}
	container := actual.Spec.Containers[0]
	if container.Name != "custom-build" {
		t.Errorf("Expected custom-build, but got %s!", container.Name)
	}
	if container.ImagePullPolicy != kapi.PullIfNotPresent {
		t.Errorf("Expected %v, got %v", kapi.PullIfNotPresent, container.ImagePullPolicy)
	}
	if actual.Spec.RestartPolicy != kapi.RestartPolicyNever {
		t.Errorf("Expected never, got %#v", actual.Spec.RestartPolicy)
	}
	if len(container.VolumeMounts) != 3 {
		t.Fatalf("Expected 3 volumes in container, got %d", len(container.VolumeMounts))
	}
	for i, expected := range []string{dockerSocketPath, DockerPushSecretMountPath, sourceSecretMountPath} {
		if container.VolumeMounts[i].MountPath != expected {
			t.Fatalf("Expected %s in VolumeMount[%d], got %s", expected, i, container.VolumeMounts[i].MountPath)
		}
	}
	if !kapi.Semantic.DeepEqual(container.Resources, expected.Spec.Resources) {
		t.Fatalf("Expected actual=expected, %v != %v", container.Resources, expected.Spec.Resources)
	}
	if len(actual.Spec.Volumes) != 3 {
		t.Fatalf("Expected 3 volumes in Build pod, got %d", len(actual.Spec.Volumes))
	}
	buildJSON, _ := latest.Codec.Encode(expected)
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

	expectedForcePull := mockCustomBuild(true)
	actualForcePull, fperr := strategy.CreateBuildPod(expectedForcePull)
	if fperr != nil {
		t.Fatalf("Unexpected error: %v", fperr)
	}
	containerForcePull := actualForcePull.Spec.Containers[0]
	if containerForcePull.ImagePullPolicy != kapi.PullAlways {
		t.Errorf("Expected %v, got %v", kapi.PullAlways, container.ImagePullPolicy)
	}
}

func mockCustomBuild(forcePull bool) *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "customBuild",
			Labels: map[string]string{
				"name": "customBuild",
			},
		},
		Spec: buildapi.BuildSpec{
			Revision: &buildapi.SourceRevision{
				Git: &buildapi.GitSourceRevision{},
			},
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://my.build.com/the/dockerbuild/Dockerfile",
					Ref: "master",
				},
				SourceSecret: &kapi.LocalObjectReference{Name: "secretFoo"},
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.CustomBuildStrategyType,
				CustomStrategy: &buildapi.CustomBuildStrategy{
					From: kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "builder-image",
					},
					Env: []kapi.EnvVar{
						{Name: "FOO", Value: "BAR"},
					},
					ExposeDockerSocket: true,
					ForcePull:          forcePull,
				},
			},
			Output: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "docker-registry/repository/customBuild",
				},
				PushSecret: &kapi.LocalObjectReference{Name: "foo"},
			},
			Resources: kapi.ResourceRequirements{
				Limits: kapi.ResourceList{
					kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
					kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("10G"),
				},
			},
		},
		Status: buildapi.BuildStatus{
			Phase: buildapi.BuildPhaseNew,
		},
	}
}
