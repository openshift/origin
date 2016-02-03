package strategy

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/build/api/install"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

func TestCustomCreateBuildPod(t *testing.T) {
	strategy := CustomBuildStrategy{
		Codec: kapi.Codecs.LegacyCodec(buildapi.SchemeGroupVersion),
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
	if *actual.Spec.ActiveDeadlineSeconds != 60 {
		t.Errorf("Expected ActiveDeadlineSeconds 60, got %d", *actual.Spec.ActiveDeadlineSeconds)
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
	buildJSON, _ := runtime.Encode(kapi.Codecs.LegacyCodec(buildapi.SchemeGroupVersion), expected)
	errorCases := map[int][]string{
		0: {"BUILD", string(buildJSON)},
	}
	standardEnv := []string{"SOURCE_REPOSITORY", "SOURCE_CONTEXT_DIR", "SOURCE_REF", "OUTPUT_IMAGE", "OUTPUT_REGISTRY"}
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

func TestCustomCreateBuildPodExpectedForcePull(t *testing.T) {
	strategy := CustomBuildStrategy{
		Codec: kapi.Codecs.LegacyCodec(buildapi.SchemeGroupVersion),
	}

	expected := mockCustomBuild(true)
	actual, fperr := strategy.CreateBuildPod(expected)
	if fperr != nil {
		t.Fatalf("Unexpected error: %v", fperr)
	}
	container := actual.Spec.Containers[0]
	if container.ImagePullPolicy != kapi.PullAlways {
		t.Errorf("Expected %v, got %v", kapi.PullAlways, container.ImagePullPolicy)
	}
}

func TestCustomCreateBuildPodWithCustomCodec(t *testing.T) {
	strategy := CustomBuildStrategy{
		Codec: kapi.Codecs.LegacyCodec(buildapi.SchemeGroupVersion),
	}

	for _, version := range registered.GroupOrDie(buildapi.GroupName).GroupVersions {
		// Create new Build specification and modify Spec API version
		build := mockCustomBuild(false)
		build.Spec.Strategy.CustomStrategy.BuildAPIVersion = fmt.Sprintf("%s/%s", version.Group, version.Version)

		pod, err := strategy.CreateBuildPod(build)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		versionFound := false
		for _, envVar := range pod.Spec.Containers[0].Env {
			if envVar.Name == "BUILD" {
				if strings.Contains(envVar.Value, fmt.Sprintf(`"apiVersion":"%s"`, version)) {
					versionFound = true
					break
				}
				t.Fatalf("BUILD environment variable doesn't contain correct API version")
			}
		}
		if !versionFound {
			t.Fatalf("Couldn't find BUILD environment variable in pod spec")
		}
	}
}

func mockCustomBuild(forcePull bool) *buildapi.Build {
	timeout := int64(60)
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
				Git: &buildapi.GitBuildSource{
					URI: "http://my.build.com/the/dockerbuild/Dockerfile",
					Ref: "master",
				},
				ContextDir:   "foo",
				SourceSecret: &kapi.LocalObjectReference{Name: "secretFoo"},
			},
			Strategy: buildapi.BuildStrategy{
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
			CompletionDeadlineSeconds: &timeout,
		},
		Status: buildapi.BuildStatus{
			Phase: buildapi.BuildPhaseNew,
		},
	}
}
