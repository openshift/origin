package apitesting

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestFieldLabelConversions(t *testing.T, scheme *runtime.Scheme, version, kind string, expectedLabels map[string]string, customLabels ...string) {
	for label := range expectedLabels {
		_, _, err := scheme.ConvertFieldLabel(version, kind, label, "")
		if err != nil {
			t.Errorf("No conversion registered for %s for %s %s", label, version, kind)
		}
	}
	for _, label := range customLabels {
		_, _, err := scheme.ConvertFieldLabel(version, kind, label, "")
		if err != nil {
			t.Errorf("No conversion registered for %s for %s %s", label, version, kind)
		}
	}
}
