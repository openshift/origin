package admission

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

	u "github.com/openshift/origin/pkg/build/admission/testutil"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

func TestIsBuildPod(t *testing.T) {
	tests := []struct {
		pod      *u.TestPod
		expected bool
	}{
		{
			pod:      u.Pod().WithAnnotation("foo", "bar"),
			expected: false,
		},
		{
			pod:      u.Pod().WithEnvVar("BUILD", "blah"),
			expected: false,
		},
		{
			pod:      u.Pod().WithAnnotation(buildapi.BuildAnnotation, "build"),
			expected: false,
		},
		{
			pod: u.Pod().
				WithAnnotation(buildapi.BuildAnnotation, "build").
				WithEnvVar("BUILD", "true"),
			expected: true,
		},
	}

	for _, tc := range tests {
		actual := IsBuildPod(tc.pod.ToAttributes())
		if actual != tc.expected {
			t.Errorf("unexpected result (%v) for pod %#v", actual, tc.pod)
		}
	}
}

func TestGetBuild(t *testing.T) {
	build := u.Build().WithDockerStrategy()
	for _, version := range []string{"v1"} {
		pod := u.Pod().WithBuild(t, build.AsBuild(), version)
		resultBuild, resultVersion, err := GetBuild(pod.ToAttributes())
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
		err = SetBuild(pod.ToAttributes(), build.AsBuild(), groupVersion)
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
	SetBuildLogLevel(pod.ToAttributes(), build.AsBuild())

	if len(pod.Spec.Containers[0].Args) == 0 {
		t.Errorf("Builds pod loglevel was not set")
	}

	if pod.Spec.Containers[0].Args[0] != "--loglevel=0" {
		t.Errorf("Default build pod loglevel was not set to 0")
	}

	build = u.Build().WithSourceStrategy()
	pod = u.Pod().WithEnvVar("BUILD", "foo")
	build.Spec.Strategy.SourceStrategy.Env = []kapi.EnvVar{{"BUILD_LOGLEVEL", "7", nil}}
	SetBuildLogLevel(pod.ToAttributes(), build.AsBuild())

	if pod.Spec.Containers[0].Args[0] != "--loglevel=7" {
		t.Errorf("Build pod loglevel was not transferred from BUILD_LOGLEVEL environment variable: %#v", pod)
	}

}
