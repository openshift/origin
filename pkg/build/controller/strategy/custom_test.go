package strategy

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"

	buildv1 "github.com/openshift/api/build/v1"
	_ "github.com/openshift/origin/pkg/build/apis/build/install"
	"github.com/openshift/origin/pkg/build/buildapihelpers"
	"github.com/openshift/origin/pkg/build/util"
)

func TestCustomCreateBuildPod(t *testing.T) {
	strategy := CustomBuildStrategy{}

	expectedBad := mockCustomBuild(false, false)
	expectedBad.Spec.Strategy.CustomStrategy.From = corev1.ObjectReference{
		Kind: "DockerImage",
		Name: "",
	}
	if _, err := strategy.CreateBuildPod(expectedBad); err == nil {
		t.Errorf("Expected error when Image is empty, got nothing")
	}

	build := mockCustomBuild(false, false)
	actual, err := strategy.CreateBuildPod(build)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if expected, actual := buildapihelpers.GetBuildPodName(build), actual.ObjectMeta.Name; expected != actual {
		t.Errorf("Expected %s, but got %s!", expected, actual)
	}
	if !reflect.DeepEqual(map[string]string{util.BuildLabel: buildapihelpers.LabelValue(build.Name)}, actual.Labels) {
		t.Errorf("Pod Labels does not match Build Labels!")
	}
	if !reflect.DeepEqual(nodeSelector, actual.Spec.NodeSelector) {
		t.Errorf("Pod NodeSelector does not match Build NodeSelector.  Expected: %v, got: %v", nodeSelector, actual.Spec.NodeSelector)
	}

	container := actual.Spec.Containers[0]
	if container.Name != CustomBuild {
		t.Errorf("Expected %s, but got %s!", CustomBuild, container.Name)
	}
	if container.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Errorf("Expected %v, got %v", corev1.PullIfNotPresent, container.ImagePullPolicy)
	}
	if actual.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("Expected never, got %#v", actual.Spec.RestartPolicy)
	}
	if len(container.VolumeMounts) != 4 {
		t.Fatalf("Expected 4 volumes in container, got %d", len(container.VolumeMounts))
	}
	if *actual.Spec.ActiveDeadlineSeconds != 60 {
		t.Errorf("Expected ActiveDeadlineSeconds 60, got %d", *actual.Spec.ActiveDeadlineSeconds)
	}
	for i, expected := range []string{dockerSocketPath, DockerPushSecretMountPath, sourceSecretMountPath} {
		if container.VolumeMounts[i].MountPath != expected {
			t.Fatalf("Expected %s in VolumeMount[%d], got %s", expected, i, container.VolumeMounts[i].MountPath)
		}
	}
	if !kapihelper.Semantic.DeepEqual(container.Resources, build.Spec.Resources) {
		t.Fatalf("Expected actual=expected, %v != %v", container.Resources, build.Spec.Resources)
	}
	if len(actual.Spec.Volumes) != 4 {
		t.Fatalf("Expected 4 volumes in Build pod, got %d", len(actual.Spec.Volumes))
	}
	buildJSON, _ := runtime.Encode(customBuildEncodingCodecFactory.LegacyCodec(buildv1.GroupVersion), build)
	errorCases := map[int][]string{
		0: {"BUILD", string(buildJSON)},
	}
	standardEnv := []string{"SOURCE_REPOSITORY", "SOURCE_URI", "SOURCE_CONTEXT_DIR", "SOURCE_REF", "OUTPUT_IMAGE", "OUTPUT_REGISTRY"}
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

	checkAliasing(t, actual)
}

func TestCustomCreateBuildPodExpectedForcePull(t *testing.T) {
	strategy := CustomBuildStrategy{}

	expected := mockCustomBuild(true, false)
	actual, fperr := strategy.CreateBuildPod(expected)
	if fperr != nil {
		t.Fatalf("Unexpected error: %v", fperr)
	}
	container := actual.Spec.Containers[0]
	if container.ImagePullPolicy != corev1.PullAlways {
		t.Errorf("Expected %v, got %v", corev1.PullAlways, container.ImagePullPolicy)
	}
}

func TestEmptySource(t *testing.T) {
	strategy := CustomBuildStrategy{}

	expected := mockCustomBuild(false, true)
	_, fperr := strategy.CreateBuildPod(expected)
	if fperr != nil {
		t.Fatalf("Unexpected error: %v", fperr)
	}
}

func TestCustomCreateBuildPodWithCustomCodec(t *testing.T) {
	strategy := CustomBuildStrategy{}

	for _, version := range []schema.GroupVersion{{Group: "", Version: "v1"}, {Group: "build.openshift.io", Version: "v1"}} {
		// Create new Build specification and modify Spec API version
		build := mockCustomBuild(false, false)
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

func TestCustomBuildLongName(t *testing.T) {
	strategy := CustomBuildStrategy{}
	build := mockCustomBuild(false, false)
	build.Name = strings.Repeat("a", validation.DNS1123LabelMaxLength*2)
	pod, err := strategy.CreateBuildPod(build)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if pod.Labels[util.BuildLabel] != build.Name[:validation.DNS1123LabelMaxLength] {
		t.Errorf("Unexpected build label value: %s", pod.Labels[util.BuildLabel])
	}
}

func mockCustomBuild(forcePull, emptySource bool) *buildv1.Build {
	timeout := int64(60)
	src := buildv1.BuildSource{}
	if !emptySource {
		src = buildv1.BuildSource{
			Git: &buildv1.GitBuildSource{
				URI: "http://my.build.com/the/dockerbuild/Dockerfile",
				Ref: "master",
			},
			ContextDir:   "foo",
			SourceSecret: &corev1.LocalObjectReference{Name: "secretFoo"},
		}
	}
	return &buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "CustomBuild",
			Labels: map[string]string{
				"name": "CustomBuild",
			},
		},
		Spec: buildv1.BuildSpec{
			CommonSpec: buildv1.CommonSpec{
				Revision: &buildv1.SourceRevision{
					Git: &buildv1.GitSourceRevision{},
				},
				Source: src,
				Strategy: buildv1.BuildStrategy{
					CustomStrategy: &buildv1.CustomBuildStrategy{
						From: corev1.ObjectReference{
							Kind: "DockerImage",
							Name: "builder-image",
						},
						Env: []corev1.EnvVar{
							{Name: "FOO", Value: "BAR"},
						},
						ExposeDockerSocket: true,
						ForcePull:          forcePull,
						Secrets: []buildv1.SecretSpec{
							{
								SecretSource: corev1.LocalObjectReference{
									Name: "secret",
								},
								MountPath: "secret",
							},
						},
					},
				},
				Output: buildv1.BuildOutput{
					To: &corev1.ObjectReference{
						Kind: "DockerImage",
						Name: "docker-registry.io/repository/custombuild",
					},
					PushSecret: &corev1.LocalObjectReference{Name: "foo"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("10"),
						corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("10G"),
					},
				},
				CompletionDeadlineSeconds: &timeout,
				NodeSelector:              nodeSelector,
			},
		},
		Status: buildv1.BuildStatus{
			Phase: buildv1.BuildPhaseNew,
		},
	}
}
