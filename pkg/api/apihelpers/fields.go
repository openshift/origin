package apihelpers

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// LegacyMetaV1FieldSelectorConversionWithName auto-accepts metav1 values for name and namespace AND "name"
// which many of our older resources used.
func LegacyMetaV1FieldSelectorConversionWithName(label, value string) (string, string, error) {
	switch label {
	case "name":
		return "metadata.name", value, nil
	default:
		return runtime.DefaultMetaV1FieldSelectorConversion(label, value)
	}
}
