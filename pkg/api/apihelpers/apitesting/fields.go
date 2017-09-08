package apitesting

import (
	"testing"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// FieldKeyCheck gathers information to check if the field key conversions are working correctly.  It takes many parameters
// in an attempt to reflect reality
type FieldKeyCheck struct {
	SchemeBuilder            runtime.SchemeBuilder
	Kind                     schema.GroupVersionKind
	AllowedExternalFieldKeys []string
	FieldKeyEvaluatorFn      FieldKeyEvaluator
}

func (f FieldKeyCheck) Check(t *testing.T) {
	scheme := runtime.NewScheme()
	f.SchemeBuilder.AddToScheme(scheme)
	internalObj, err := scheme.New(f.Kind.GroupKind().WithVersion(runtime.APIVersionInternal))
	if err != nil {
		t.Errorf("unable to new up %v", f.Kind)
	}

	for _, externalFieldKey := range f.AllowedExternalFieldKeys {
		internalFieldKey, _, err := scheme.ConvertFieldLabel(f.Kind.GroupVersion().String(), f.Kind.Kind, externalFieldKey, "")
		if err != nil {
			t.Errorf("illegal field conversion %q for %v", externalFieldKey, f.Kind)
			continue
		}

		fieldSet := fields.Set{}
		if err := f.FieldKeyEvaluatorFn(internalObj, fieldSet); err != nil {
			t.Errorf("unable to valuate field keys for %v: %v", f.Kind, err)
			continue
		}

		found := false
		for actualInternalFieldKey := range fieldSet {
			if internalFieldKey == actualInternalFieldKey {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%q converted to %q which has no internal field key match for %v", externalFieldKey, internalFieldKey, f.Kind)
			continue
		}

	}

}

// FieldKeyEvaluator overlaps with the storage mutation func.  We use this to confirm that the non-meta fields are actually being handled
type FieldKeyEvaluator func(obj runtime.Object, fieldSet fields.Set) error
