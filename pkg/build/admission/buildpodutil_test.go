package admission

import (
	"reflect"
	"testing"

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
