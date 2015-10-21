package util

import (
	"testing"

	buildapi "github.com/openshift/origin/pkg/build/api"
	kapi "k8s.io/kubernetes/pkg/api"
)

func TestGetBuildPodName(t *testing.T) {
	if expected, actual := "mybuild-build", GetBuildPodName(&buildapi.Build{ObjectMeta: kapi.ObjectMeta{Name: "mybuild"}}); expected != actual {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}
