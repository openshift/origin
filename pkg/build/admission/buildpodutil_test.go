package admission

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

	u "github.com/openshift/origin/pkg/build/admission/testutil"
)

func TestGetBuild(t *testing.T) {
	build := u.Build().WithDockerStrategy()
	for _, version := range []string{"v1"} {
		pod := u.Pod().WithBuild(t, build.AsBuild(), version)
		resultBuild, resultVersion, err := GetBuildFromPod((*kapi.Pod)(pod))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resultVersion.Version != version {
			t.Errorf("unexpected version: %s", resultVersion)
		}
		if !reflect.DeepEqual(build.AsBuild(), resultBuild) {
			t.Errorf("%s: did not get expected build: %#v", version, resultBuild)
		}
	}
}

func TestSetBuild(t *testing.T) {
	build := u.Build().WithSourceStrategy()
	for _, version := range []string{"v1"} {
		pod := u.Pod().WithEnvVar("BUILD", "foo")
		groupVersion, err := unversioned.ParseGroupVersion(version)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		err = SetBuildInPod((*kapi.Pod)(pod), build.AsBuild(), groupVersion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resultBuild := pod.GetBuild(t)
		if !reflect.DeepEqual(build.AsBuild(), resultBuild) {
			t.Errorf("%s: did not get expected build: %#v", version, resultBuild)
		}
	}
}

func TestSetBuildLogLevel(t *testing.T) {
	build := u.Build().WithSourceStrategy()
	pod := u.Pod().WithEnvVar("BUILD", "foo")
	SetPodLogLevelFromBuild((*kapi.Pod)(pod), build.AsBuild())

	if len(pod.Spec.Containers[0].Args) == 0 {
		t.Errorf("Builds pod loglevel was not set")
	}

	if pod.Spec.Containers[0].Args[0] != "--loglevel=0" {
		t.Errorf("Default build pod loglevel was not set to 0")
	}

	build = u.Build().WithSourceStrategy()
	pod = u.Pod().WithEnvVar("BUILD", "foo")
	build.Spec.Strategy.SourceStrategy.Env = []kapi.EnvVar{{Name: "BUILD_LOGLEVEL", Value: "7", ValueFrom: nil}}
	SetPodLogLevelFromBuild((*kapi.Pod)(pod), build.AsBuild())

	if pod.Spec.Containers[0].Args[0] != "--loglevel=7" {
		t.Errorf("Build pod loglevel was not transferred from BUILD_LOGLEVEL environment variable: %#v", pod)
	}

}
