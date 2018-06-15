package v1

import (
	"github.com/openshift/origin/pkg/api/apihelpers"
	"k8s.io/apimachinery/pkg/runtime"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	return nil
}

func addLegacyFieldSelectorKeyConversions(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(LegacySchemeGroupVersion.String(), "Project", legacyProjectFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	return nil
}

func addFieldSelectorKeyConversions(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.String(), "Project", projectFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	return nil
}

// because field selectors can vary in support by version they are exposed under, we have one function for each
// groupVersion we're registering for
// we don't actually do any evaluation, only passing through, so we don't have our own field selector to test.  The upstream
// cannot remove the field selectors or they break compatibility, so we're fine.

func legacyProjectFieldSelectorKeyConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "status.phase":
		return label, value, nil
	default:
		return apihelpers.LegacyMetaV1FieldSelectorConversionWithName(label, value)
	}
}

func projectFieldSelectorKeyConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "status.phase":
		return label, value, nil
	default:
		return runtime.DefaultMetaV1FieldSelectorConversion(label, value)
	}
}
