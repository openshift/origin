package admission

import (
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	u "github.com/openshift/origin/pkg/build/admission/testutil"

	_ "github.com/openshift/origin/pkg/build/apis/build/install"
)

func TestGetBuild(t *testing.T) {
	build := u.Build().WithDockerStrategy()
	for _, version := range []string{"v1"} {
		pod := u.Pod().WithBuild(t, build.AsBuild(), version)
		resultBuild, resultVersion, err := GetBuildFromPod((*v1.Pod)(pod))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resultVersion.Version != version {
			t.Errorf("unexpected version: %s", resultVersion)
		}
		if e, a := build.AsBuild(), resultBuild; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: did not get expected build: %s", version, diff.ObjectDiff(e, a))
		}
	}
}

func TestSetBuild(t *testing.T) {
	build := u.Build().WithSourceStrategy()
	for _, version := range []string{"v1"} {
		pod := u.Pod().WithEnvVar("BUILD", "foo")
		groupVersion, err := schema.ParseGroupVersion(version)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		err = SetBuildInPod((*v1.Pod)(pod), build.AsBuild(), groupVersion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resultBuild := pod.GetBuild(t)
		if e, a := build.AsBuild(), resultBuild; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: did not get expected build: %s", version, diff.ObjectDiff(e, a))
		}
	}
}

func TestSetBuildLogLevel(t *testing.T) {
	build := u.Build().WithSourceStrategy()
	pod := u.Pod().WithEnvVar("BUILD", "foo")
	SetPodLogLevelFromBuild((*v1.Pod)(pod), build.AsBuild())

	if len(pod.Spec.Containers[0].Args) == 0 {
		t.Errorf("Builds pod loglevel was not set")
	}

	if pod.Spec.Containers[0].Args[0] != "--loglevel=0" {
		t.Errorf("Default build pod loglevel was not set to 0")
	}

	build = u.Build().WithSourceStrategy()
	pod = u.Pod().WithEnvVar("BUILD", "foo")
	build.Spec.Strategy.SourceStrategy.Env = []kapi.EnvVar{{Name: "BUILD_LOGLEVEL", Value: "7", ValueFrom: nil}}
	SetPodLogLevelFromBuild((*v1.Pod)(pod), build.AsBuild())

	if pod.Spec.Containers[0].Args[0] != "--loglevel=7" {
		t.Errorf("Build pod loglevel was not transferred from BUILD_LOGLEVEL environment variable: %#v", pod)
	}

}
