package clientcmd

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
)

// TestRunGenerators makes sure we catch new generators added to `oc run`
func TestRunGenerators(t *testing.T) {
	f := NewFactory(nil)

	// Contains the run generators we expect to see
	expectedRunGenerators := sets.NewString(
		// kube generators
		"run/v1",
		"run-pod/v1",
		"deployment/apps.v1beta1",
		"deployment/v1beta1",
		"job/v1",
		"cronjob/v2alpha1",
		"cronjob/v1beta1",

		// origin generators
		"run-controller/v1", // legacy alias for run/v1
		"deploymentconfig/v1",
	).List()

	runGenerators := sets.StringKeySet(f.Generators("run")).List()
	if !reflect.DeepEqual(expectedRunGenerators, runGenerators) {
		t.Errorf("Expected run generators:%#v, got:\n%#v", expectedRunGenerators, runGenerators)
	}
}
