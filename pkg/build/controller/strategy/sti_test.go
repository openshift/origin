package strategy

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation"

	buildapi "github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/build/api/install"
)

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

var nodeSelector = map[string]string{"node": "mynode"}

func testSTICreateBuildPod(t *testing.T, rootAllowed bool) {
	strategy := &SourceBuildStrategy{
		Image:            "sti-test-image",
		Codec:            kapi.Codecs.LegacyCodec(buildapi.SchemeGroupVersion),
		AdmissionControl: &FakeAdmissionControl{admit: rootAllowed},
	}

	build := mockSTIBuild()
	actual, err := strategy.CreateBuildPod(build)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if expected, actual := buildapi.GetBuildPodName(build), actual.ObjectMeta.Name; expected != actual {
		t.Errorf("Expected %s, but got %s!", expected, actual)
	}
	if !reflect.DeepEqual(map[string]string{buildapi.BuildLabel: buildapi.LabelValue(build.Name)}, actual.Labels) {
		t.Errorf("Pod Labels does not match Build Labels!")
	}
	if !reflect.DeepEqual(nodeSelector, actual.Spec.NodeSelector) {
		t.Errorf("Pod NodeSelector does not match Build NodeSelector.  Expected: %v, got: %v", nodeSelector, actual.Spec.NodeSelector)
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
	// the values are allowed, so only expect 10 not 11 values.
	expectedEnvCount := 10
	if !rootAllowed {
		expectedEnvCount = 12
	}
	if len(container.Env) != expectedEnvCount {
		var keys []string
		for _, env := range container.Env {
			keys = append(keys, env.Name)
		}
		t.Fatalf("Expected 11 elements in Env table, got %d:\n%s", len(container.Env), strings.Join(keys, ", "))
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
	if *actual.Spec.ActiveDeadlineSeconds != 60 {
		t.Errorf("Expected ActiveDeadlineSeconds 60, got %d", *actual.Spec.ActiveDeadlineSeconds)
	}
	if !kapi.Semantic.DeepEqual(container.Resources, build.Spec.Resources) {
		t.Fatalf("Expected actual=expected, %v != %v", container.Resources, build.Spec.Resources)
	}
	found := false
	foundIllegal := false
	foundAllowedUIDs := false
	foundDropCaps := false
	for _, v := range container.Env {
		if v.Name == "BUILD_LOGLEVEL" && v.Value == "bar" {
			found = true
		}
		if v.Name == "ILLEGAL" {
			foundIllegal = true
		}
		if v.Name == buildapi.AllowedUIDs && v.Value == "1-" {
			foundAllowedUIDs = true
		}
		if v.Name == buildapi.DropCapabilities && v.Value == "KILL,MKNOD,SETGID,SETUID,SYS_CHROOT" {
			foundDropCaps = true
		}
	}
	if !found {
		t.Fatalf("Expected variable BUILD_LOGLEVEL be defined for the container")
	}
	if foundIllegal {
		t.Fatalf("Found illegal environment variable 'ILLEGAL' defined on container")
	}
	if foundAllowedUIDs && rootAllowed {
		t.Fatalf("Did not expect %s when root is allowed", buildapi.AllowedUIDs)
	}
	if !foundAllowedUIDs && !rootAllowed {
		t.Fatalf("Expected %s when root is not allowed", buildapi.AllowedUIDs)
	}
	if foundDropCaps && rootAllowed {
		t.Fatalf("Did not expect %s when root is allowed", buildapi.DropCapabilities)
	}
	if !foundDropCaps && !rootAllowed {
		t.Fatalf("Expected %s when root is not allowed", buildapi.DropCapabilities)
	}
	buildJSON, _ := runtime.Encode(kapi.Codecs.LegacyCodec(buildapi.SchemeGroupVersion), build)
	errorCases := map[int][]string{
		0: {"BUILD", string(buildJSON)},
	}
	for index, exp := range errorCases {
		if e := container.Env[index]; e.Name != exp[0] || e.Value != exp[1] {
			t.Errorf("Expected %s:%s, got %s:%s!\n", exp[0], exp[1], e.Name, e.Value)
		}
	}
}

func TestS2IBuildLongName(t *testing.T) {
	strategy := &SourceBuildStrategy{
		Image:            "sti-test-image",
		Codec:            kapi.Codecs.LegacyCodec(buildapi.SchemeGroupVersion),
		AdmissionControl: &FakeAdmissionControl{admit: true},
	}
	build := mockSTIBuild()
	build.Name = strings.Repeat("a", validation.DNS1123LabelMaxLength*2)
	pod, err := strategy.CreateBuildPod(build)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if pod.Labels[buildapi.BuildLabel] != build.Name[:validation.DNS1123LabelMaxLength] {
		t.Errorf("Unexpected build label value: %s", pod.Labels[buildapi.BuildLabel])
	}
}

func mockSTIBuild() *buildapi.Build {
	timeout := int64(60)
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "stiBuild",
			Labels: map[string]string{
				"name": "stiBuild",
			},
		},
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
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
				CompletionDeadlineSeconds: &timeout,
				NodeSelector:              nodeSelector,
			},
		},
		Status: buildapi.BuildStatus{
			Phase: buildapi.BuildPhaseNew,
		},
	}
}
