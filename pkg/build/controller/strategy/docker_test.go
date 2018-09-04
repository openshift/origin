package strategy

import (
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"

	buildv1 "github.com/openshift/api/build/v1"
	_ "github.com/openshift/origin/pkg/build/apis/build/install"
	"github.com/openshift/origin/pkg/build/buildapihelpers"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

func TestDockerCreateBuildPod(t *testing.T) {
	strategy := DockerBuildStrategy{
		Image: "docker-test-image",
	}

	build := mockDockerBuild()
	actual, err := strategy.CreateBuildPod(build)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if expected, actual := buildapihelpers.GetBuildPodName(build), actual.ObjectMeta.Name; expected != actual {
		t.Errorf("Expected %s, but got %s!", expected, actual)
	}
	if !reflect.DeepEqual(map[string]string{buildutil.BuildLabel: buildapihelpers.LabelValue(build.Name)}, actual.Labels) {
		t.Errorf("Pod Labels does not match Build Labels!")
	}
	if !reflect.DeepEqual(nodeSelector, actual.Spec.NodeSelector) {
		t.Errorf("Pod NodeSelector does not match Build NodeSelector.  Expected: %v, got: %v", nodeSelector, actual.Spec.NodeSelector)
	}

	container := actual.Spec.Containers[0]
	if container.Name != DockerBuild {
		t.Errorf("Expected %s, but got %s!", DockerBuild, container.Name)
	}
	if container.Image != strategy.Image {
		t.Errorf("Expected %s image, got %s!", container.Image, strategy.Image)
	}
	if container.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Errorf("Expected %v, got %v", corev1.PullIfNotPresent, container.ImagePullPolicy)
	}
	if actual.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("Expected never, got %#v", actual.Spec.RestartPolicy)
	}
	expectedKeys := map[string]string{"BUILD": "", "SOURCE_REPOSITORY": "", "SOURCE_URI": "", "SOURCE_CONTEXT_DIR": "", "SOURCE_REF": "", "BUILD_LOGLEVEL": "", "PUSH_DOCKERCFG_PATH": "", "PULL_DOCKERCFG_PATH": ""}
	gotKeys := map[string]string{}
	for _, k := range container.Env {
		gotKeys[k.Name] = ""
	}
	if !reflect.DeepEqual(expectedKeys, gotKeys) {
		t.Errorf("Expected environment keys:\n%v\ngot keys\n%v", expectedKeys, gotKeys)
	}

	// the pod has 6 volumes but the git source secret is not mounted into the main container.
	if len(container.VolumeMounts) != 7 {
		t.Fatalf("Expected 7 volumes in container, got %d", len(container.VolumeMounts))
	}
	if *actual.Spec.ActiveDeadlineSeconds != 60 {
		t.Errorf("Expected ActiveDeadlineSeconds 60, got %d", *actual.Spec.ActiveDeadlineSeconds)
	}
	for i, expected := range []string{buildutil.BuildWorkDirMount, dockerSocketPath, "/var/run/crio/crio.sock", DockerPushSecretMountPath, DockerPullSecretMountPath} {
		if container.VolumeMounts[i].MountPath != expected {
			t.Fatalf("Expected %s in VolumeMount[%d], got %s", expected, i, container.VolumeMounts[i].MountPath)
		}
	}
	if len(actual.Spec.Volumes) != 8 {
		t.Fatalf("Expected 8 volumes in Build pod, got %d", len(actual.Spec.Volumes))
	}
	if !kapihelper.Semantic.DeepEqual(container.Resources, build.Spec.Resources) {
		t.Fatalf("Expected actual=expected, %v != %v", container.Resources, build.Spec.Resources)
	}
	found := false
	foundIllegal := false
	for _, v := range container.Env {
		if v.Name == "BUILD_LOGLEVEL" && v.Value == "bar" {
			found = true
		}
		if v.Name == "ILLEGAL" {
			foundIllegal = true
		}
	}
	if !found {
		t.Fatalf("Expected variable BUILD_LOGLEVEL be defined for the container")
	}
	if foundIllegal {
		t.Fatalf("Found illegal environment variable 'ILLEGAL' defined on container")
	}

	buildJSON, _ := runtime.Encode(buildJSONCodec, build)
	errorCases := map[int][]string{
		0: {"BUILD", string(buildJSON)},
	}
	for index, exp := range errorCases {
		if e := container.Env[index]; e.Name != exp[0] || e.Value != exp[1] {
			t.Errorf("Expected %s:%s, got %s:%s!\n", exp[0], exp[1], e.Name, e.Value)
		}
	}

	checkAliasing(t, actual)
}

func TestDockerBuildLongName(t *testing.T) {
	strategy := DockerBuildStrategy{
		Image: "docker-test-image",
	}
	build := mockDockerBuild()
	build.Name = strings.Repeat("a", validation.DNS1123LabelMaxLength*2)
	pod, err := strategy.CreateBuildPod(build)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if pod.Labels[buildutil.BuildLabel] != build.Name[:validation.DNS1123LabelMaxLength] {
		t.Errorf("Unexpected build label value: %s", pod.Labels[buildutil.BuildLabel])
	}
}

func mockDockerBuild() *buildv1.Build {
	timeout := int64(60)
	return &buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dockerBuild",
			Labels: map[string]string{
				"name": "dockerBuild",
			},
		},
		Spec: buildv1.BuildSpec{
			CommonSpec: buildv1.CommonSpec{
				Revision: &buildv1.SourceRevision{
					Git: &buildv1.GitSourceRevision{},
				},
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						URI: "http://my.build.com/the/dockerbuild/Dockerfile",
						Ref: "master",
					},
					ContextDir:   "my/test/dir",
					SourceSecret: &corev1.LocalObjectReference{Name: "secretFoo"},
					Secrets: []buildv1.SecretBuildSource{
						{
							Secret: corev1.LocalObjectReference{
								Name: "super-secret",
							},
							DestinationDir: "a/path/for/secret",
						},
					},
					ConfigMaps: []buildv1.ConfigMapBuildSource{
						{
							ConfigMap: corev1.LocalObjectReference{
								Name: "build-config",
							},
							DestinationDir: "a/path/for/config",
						},
					},
				},
				Strategy: buildv1.BuildStrategy{
					DockerStrategy: &buildv1.DockerBuildStrategy{
						PullSecret: &corev1.LocalObjectReference{Name: "bar"},
						Env: []corev1.EnvVar{
							{Name: "ILLEGAL", Value: "foo"},
							{Name: "BUILD_LOGLEVEL", Value: "bar"},
						},
					},
				},
				Output: buildv1.BuildOutput{
					To: &corev1.ObjectReference{
						Kind: "DockerImage",
						Name: "docker-registry/repository/dockerBuild",
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
