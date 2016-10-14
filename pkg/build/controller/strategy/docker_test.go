package strategy

import (
	"reflect"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation"

	buildapi "github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/build/api/install"
)

func TestDockerCreateBuildPod(t *testing.T) {
	strategy := DockerBuildStrategy{
		Image: "docker-test-image",
		Codec: kapi.Codecs.LegacyCodec(buildapi.SchemeGroupVersion),
	}

	build := mockDockerBuild()
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
	if len(container.Env) != 10 {
		var keys []string
		for _, env := range container.Env {
			keys = append(keys, env.Name)
		}
		t.Fatalf("Expected 10 elements in Env table, got %d:\n%s", len(container.Env), strings.Join(keys, ", "))
	}
	if len(container.VolumeMounts) != 4 {
		t.Fatalf("Expected 4 volumes in container, got %d", len(container.VolumeMounts))
	}
	if *actual.Spec.ActiveDeadlineSeconds != 60 {
		t.Errorf("Expected ActiveDeadlineSeconds 60, got %d", *actual.Spec.ActiveDeadlineSeconds)
	}
	for i, expected := range []string{dockerSocketPath, DockerPushSecretMountPath, DockerPullSecretMountPath, sourceSecretMountPath} {
		if container.VolumeMounts[i].MountPath != expected {
			t.Fatalf("Expected %s in VolumeMount[%d], got %s", expected, i, container.VolumeMounts[i].MountPath)
		}
	}
	if len(actual.Spec.Volumes) != 4 {
		t.Fatalf("Expected 4 volumes in Build pod, got %d", len(actual.Spec.Volumes))
	}
	if !kapi.Semantic.DeepEqual(container.Resources, build.Spec.Resources) {
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

func TestDockerBuildLongName(t *testing.T) {
	strategy := DockerBuildStrategy{
		Image: "docker-test-image",
		Codec: kapi.Codecs.LegacyCodec(buildapi.SchemeGroupVersion),
	}
	build := mockDockerBuild()
	build.Name = strings.Repeat("a", validation.DNS1123LabelMaxLength*2)
	pod, err := strategy.CreateBuildPod(build)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if pod.Labels[buildapi.BuildLabel] != build.Name[:validation.DNS1123LabelMaxLength] {
		t.Errorf("Unexpected build label value: %s", pod.Labels[buildapi.BuildLabel])
	}
}

func mockDockerBuild() *buildapi.Build {
	timeout := int64(60)
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "dockerBuild",
			Labels: map[string]string{
				"name": "dockerBuild",
			},
		},
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				Revision: &buildapi.SourceRevision{
					Git: &buildapi.GitSourceRevision{},
				},
				Source: buildapi.BuildSource{
					Git: &buildapi.GitBuildSource{
						URI: "http://my.build.com/the/dockerbuild/Dockerfile",
						Ref: "master",
					},
					ContextDir:   "my/test/dir",
					SourceSecret: &kapi.LocalObjectReference{Name: "secretFoo"},
				},
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{
						PullSecret: &kapi.LocalObjectReference{Name: "bar"},
						Env: []kapi.EnvVar{
							{Name: "ILLEGAL", Value: "foo"},
							{Name: "BUILD_LOGLEVEL", Value: "bar"},
						},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "docker-registry/repository/dockerBuild",
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
