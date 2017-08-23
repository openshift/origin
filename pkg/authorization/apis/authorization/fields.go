package authorization

import "k8s.io/apimachinery/pkg/fields"

// PolicyBindingToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func PolicyBindingToSelectableFields(policyBinding *PolicyBinding) fields.Set {
	return fields.Set{
		"metadata.name":       policyBinding.Name,
		"metadata.namespace":  policyBinding.Namespace,
		"policyRef.namespace": policyBinding.PolicyRef.Namespace,
	}
}
