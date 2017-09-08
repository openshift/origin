package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

func addLegacyFieldConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(LegacySchemeGroupVersion.String(), "Route", legacyRouteFieldSelectorConversionFunc); err != nil {
		return err
	}
	return nil
}

func addFieldConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.String(), "Route", routeFieldSelectorConversionFunc); err != nil {
		return err
	}
	return nil
}

// because field selectors can vary in support by version they are exposed under, we have one function for each
// groupVersion we're registering for

func legacyRouteFieldSelectorConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "spec.path",
		"spec.host",
		"spec.to.name":
		return label, value, nil
	default:
		return runtime.DefaultMetaV1FieldSelectorConversion(label, value)
	}
}

func routeFieldSelectorConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "spec.path",
		"spec.host",
		"spec.to.name":
		return label, value, nil
	default:
		return runtime.DefaultMetaV1FieldSelectorConversion(label, value)
	}
}
