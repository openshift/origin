package strategy

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"

	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

type FakeTempDirCreator struct{}

func (t *FakeTempDirCreator) CreateTempDirectory() (string, error) {
	return "test_temp", nil
}

type FakeAdmissionControl struct {
	admit bool
}

func (a *FakeAdmissionControl) Admit(attr admission.Attributes) (err error) {
	if a.admit {
		return nil
	}
	return fmt.Errorf("pod not allowed")
}

func (a *FakeAdmissionControl) Handles(operation admission.Operation) bool {
	return true
}

func TestSTICreateBuildPodRootNotAllowed(t *testing.T) {
	testSTICreateBuildPod(t, false)
}

func TestSTICreateBuildPodRootAllowed(t *testing.T) {
	testSTICreateBuildPod(t, true)
}

func testSTICreateBuildPod(t *testing.T, rootAllowed bool) {
	strategy := &SourceBuildStrategy{
		Image:                "sti-test-image",
		TempDirectoryCreator: &FakeTempDirCreator{},
		Codec:                latest.Codec,
		AdmissionControl:     &FakeAdmissionControl{admit: rootAllowed},
	}

	expected := mockSTIBuild()
	actual, err := strategy.CreateBuildPod(expected)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if expected, actual := buildutil.GetBuildPodName(expected), actual.ObjectMeta.Name; expected != actual {
		t.Errorf("Expected %s, but got %s!", expected, actual)
	}
	if !reflect.DeepEqual(map[string]string{buildapi.BuildLabel: expected.Name}, actual.Labels) {
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
	// strategy ENV is whitelisted into the container environment, and not all
	// the values are allowed, so only expect 6 not 7 values.
	expectedEnvCount := 8
	if !rootAllowed {
		expectedEnvCount = 9
	}
	if len(container.Env) != expectedEnvCount {
		t.Fatalf("Expected 9 elements in Env table, got %d: %+v", len(container.Env), container.Env)
	}
	if len(container.VolumeMounts) != 4 {
		t.Fatalf("Expected 4 volumes in container, got %d", len(container.VolumeMounts))
	}
	for i, expected := range []string{dockerSocketPath, DockerPushSecretMountPath, DockerPullSecretMountPath, sourceSecretMountPath} {
		if container.VolumeMounts[i].MountPath != expected {
			t.Fatalf("Expected %s in VolumeMount[%d], got %s", expected, i, container.VolumeMounts[i].MountPath)
		}
	}
	if len(actual.Spec.Volumes) != 4 {
		t.Fatalf("Expected 4 volumes in Build pod, got %d", len(actual.Spec.Volumes))
	}
	if !kapi.Semantic.DeepEqual(container.Resources, expected.Spec.Resources) {
		t.Fatalf("Expected actual=expected, %v != %v", container.Resources, expected.Spec.Resources)
	}
	found := false
	foundIllegal := false
	foundAllowedUIDs := false
	for _, v := range container.Env {
		if v.Name == "BUILD_LOGLEVEL" && v.Value == "bar" {
			found = true
		}
		if v.Name == "ILLEGAL" {
			foundIllegal = true
		}
		if v.Name == "ALLOWED_UIDS" && v.Value == "1-" {
			foundAllowedUIDs = true
		}
	}
	if !found {
		t.Fatalf("Expected variable BUILD_LOGLEVEL be defined for the container")
	}
	if foundIllegal {
		t.Fatalf("Found illegal environment variable 'ILLEGAL' defined on container")
	}
	if foundAllowedUIDs && rootAllowed {
		t.Fatalf("Did not expect ALLOWED_UIDS when root is allowed")
	}
	if !foundAllowedUIDs && !rootAllowed {
		t.Fatalf("Expected ALLLOWED_UIDS when root is not allowed")
	}
	buildJSON, _ := latest.Codec.Encode(expected)
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
		Spec: buildapi.BuildSpec{
			Revision: &buildapi.SourceRevision{
				Git: &buildapi.GitSourceRevision{},
			},
			Source: buildapi.BuildSource{
				Git: &buildapi.GitBuildSource{
					URI: "http://my.build.com/the/stibuild/Dockerfile",
					Ref: "master",
				},
				ContextDir:   "foo",
				SourceSecret: &kapi.LocalObjectReference{Name: "fooSecret"},
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.SourceBuildStrategyType,
				SourceStrategy: &buildapi.SourceBuildStrategy{
					From: kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "repository/sti-builder",
					},
					PullSecret: &kapi.LocalObjectReference{Name: "bar"},
					Scripts:    "http://my.build.com/the/sti/scripts",
					Env: []kapi.EnvVar{
						{Name: "BUILD_LOGLEVEL", Value: "bar"},
						{Name: "ILLEGAL", Value: "foo"},
					},
				},
			},
			Output: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "docker-registry/repository/stiBuild",
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
