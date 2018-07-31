package common

import (
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"

	u "github.com/openshift/origin/pkg/build/controller/common/testutil"

	_ "github.com/openshift/origin/pkg/build/apis/build/install"
)

func TestGetBuild(t *testing.T) {
	build := u.Build().WithDockerStrategy()
	pod := u.Pod().WithBuild(t, build.AsBuild())
	resultBuild, err := GetBuildFromPod((*v1.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := build.AsBuild(), resultBuild; !reflect.DeepEqual(e, a) {
		t.Errorf("did not get expected build: %s", diff.ObjectDiff(e, a))
	}
}

func TestSetBuild(t *testing.T) {
	build := u.Build().WithSourceStrategy()
	version := schema.GroupVersion{Group: "build.openshift.io", Version: "v1"}
	pod := u.Pod().WithEnvVar("BUILD", "foo")
	if err := SetBuildInPod((*v1.Pod)(pod), build.AsBuild()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resultBuild := pod.GetBuild(t)
	if e, a := build.AsBuild(), resultBuild; !reflect.DeepEqual(e, a) {
		t.Errorf("%s: did not get expected build: %s", version, diff.ObjectDiff(e, a))
	}
}
