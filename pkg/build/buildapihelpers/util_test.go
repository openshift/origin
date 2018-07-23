package buildapihelpers

import (
	"testing"

	buildinternalapi "github.com/openshift/origin/pkg/build/apis/build"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetBuildPodName(t *testing.T) {
	if expected, actual := "mybuild-build", GetBuildPodName(&buildinternalapi.Build{ObjectMeta: metav1.ObjectMeta{Name: "mybuild"}}); expected != actual {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}
