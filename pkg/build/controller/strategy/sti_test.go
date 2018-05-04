package strategy

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	_ "github.com/openshift/origin/pkg/build/apis/build/install"
	"github.com/openshift/origin/pkg/build/util"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/security/apis/security"
	securityinternalversion "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
	"github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion/fake"
)

func newFakeSecurityClient(rootAllowed bool) securityinternalversion.SecurityInterface {
	securityClient := &fake.FakeSecurity{Fake: &clienttesting.Fake{}}
	securityClient.AddReactor("*", "*", func(clienttesting.Action) (bool, runtime.Object, error) {
		var ref *kapi.ObjectReference
		if rootAllowed {
			ref = &kapi.ObjectReference{} // i.e., not nil
		}

		return true, &security.PodSecurityPolicySubjectReview{
			Status: security.PodSecurityPolicySubjectReviewStatus{
				AllowedBy: ref,
			},
		}, nil
	})
	return securityClient
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
		Image:          "sti-test-image",
		Codec:          legacyscheme.Codecs.LegacyCodec(buildapi.LegacySchemeGroupVersion),
		SecurityClient: newFakeSecurityClient(rootAllowed),
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
	if container.Name != StiBuild {
		t.Errorf("Expected %s, but got %s!", StiBuild, container.Name)
	}
	if container.Image != strategy.Image {
		t.Errorf("Expected %s image, got %s!", container.Image, strategy.Image)
	}
	if container.ImagePullPolicy != v1.PullIfNotPresent {
		t.Errorf("Expected %v, got %v", v1.PullIfNotPresent, container.ImagePullPolicy)
	}
	if actual.Spec.RestartPolicy != v1.RestartPolicyNever {
		t.Errorf("Expected never, got %#v", actual.Spec.RestartPolicy)
	}

	// strategy ENV variables are whitelisted(filtered) into the container environment, and not all
	// the values are allowed, so don't expect to see the filtered values in the result.
	expectedKeys := map[string]string{"BUILD": "", "SOURCE_REPOSITORY": "", "SOURCE_URI": "", "SOURCE_CONTEXT_DIR": "", "SOURCE_REF": "", "ORIGIN_VERSION": "", "BUILD_LOGLEVEL": "", "PUSH_DOCKERCFG_PATH": "", "PULL_DOCKERCFG_PATH": ""}
	if !rootAllowed {
		expectedKeys["ALLOWED_UIDS"] = ""
		expectedKeys["DROP_CAPS"] = ""
	}
	gotKeys := map[string]string{}
	for _, k := range container.Env {
		gotKeys[k.Name] = ""
	}
	if !reflect.DeepEqual(expectedKeys, gotKeys) {
		t.Errorf("Expected environment keys:\n%v\ngot keys\n%v", expectedKeys, gotKeys)
	}

	// the pod has 6 volumes but the git source secret is not mounted into the main container.
	if len(container.VolumeMounts) != 5 {
		t.Fatalf("Expected 5 volumes in container, got %d", len(container.VolumeMounts))
	}
	for i, expected := range []string{buildutil.BuildWorkDirMount, dockerSocketPath, "/var/run/crio/crio.sock", DockerPushSecretMountPath, DockerPullSecretMountPath} {
		if container.VolumeMounts[i].MountPath != expected {
			t.Fatalf("Expected %s in VolumeMount[%d], got %s", expected, i, container.VolumeMounts[i].MountPath)
		}
	}
	if len(actual.Spec.Volumes) != 6 {
		t.Fatalf("Expected 6 volumes in Build pod, got %d", len(actual.Spec.Volumes))
	}
	if *actual.Spec.ActiveDeadlineSeconds != 60 {
		t.Errorf("Expected ActiveDeadlineSeconds 60, got %d", *actual.Spec.ActiveDeadlineSeconds)
	}
	if !kapihelper.Semantic.DeepEqual(container.Resources, util.CopyApiResourcesToV1Resources(&build.Spec.Resources)) {
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
		if v.Name == buildapi.DropCapabilities && v.Value == "KILL,MKNOD,SETGID,SETUID" {
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
	buildJSON, _ := runtime.Encode(legacyscheme.Codecs.LegacyCodec(buildapi.LegacySchemeGroupVersion), build)
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

func TestS2IBuildLongName(t *testing.T) {
	strategy := &SourceBuildStrategy{
		Image:          "sti-test-image",
		Codec:          legacyscheme.Codecs.LegacyCodec(buildapi.LegacySchemeGroupVersion),
		SecurityClient: newFakeSecurityClient(true),
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
		ObjectMeta: metav1.ObjectMeta{
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
