package apihelpers

import (
	"fmt"

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

// GetFieldLabelConversionFunc returns a field label conversion func, which does the following:
// * returns overrideLabels[label], value, nil if the specified label exists in the overrideLabels map
// * returns label, value, nil if the specified label exists as a key in the supportedLabels map (values in this map are unused, it is intended to be a prototypical label/value map)
// * otherwise, returns an error
func GetFieldLabelConversionFunc(supportedLabels map[string]string, overrideLabels map[string]string) func(label, value string) (string, string, error) {
	return func(label, value string) (string, string, error) {
		if label, overridden := overrideLabels[label]; overridden {
			return label, value, nil
		}
		if _, supported := supportedLabels[label]; supported {
			return label, value, nil
		}
		return "", "", fmt.Errorf("field label not supported: %s", label)
	}
}
