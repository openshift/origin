package api

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/registry/generic"
)

// ClusterPolicyToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func ClusterPolicyToSelectableFields(policy *ClusterPolicy) fields.Set {
	return generic.ObjectMetaFieldsSet(&policy.ObjectMeta, false)
}

// ClusterPolicyBindingToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func ClusterPolicyBindingToSelectableFields(policyBinding *ClusterPolicyBinding) fields.Set {
	return generic.ObjectMetaFieldsSet(&policyBinding.ObjectMeta, false)
}

// PolicyToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func PolicyToSelectableFields(policy *Policy) fields.Set {
	return generic.ObjectMetaFieldsSet(&policy.ObjectMeta, true)
}

// PolicyBindingToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func PolicyBindingToSelectableFields(policyBinding *PolicyBinding) fields.Set {
	return generic.AddObjectMetaFieldsSet(fields.Set{
		"policyRef.namespace": policyBinding.PolicyRef.Namespace,
	}, &policyBinding.ObjectMeta, true)
}

// RoleToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func RoleToSelectableFields(role *Role) fields.Set {
	return generic.ObjectMetaFieldsSet(&role.ObjectMeta, true)
}

// RoleBindingToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func RoleBindingToSelectableFields(roleBinding *RoleBinding) fields.Set {
	return generic.ObjectMetaFieldsSet(&roleBinding.ObjectMeta, true)
}

// RoleBindingRestrictionToSelectableFields returns a label set that be used to
// identify a RoleBindingRestriction object.
func RoleBindingRestrictionToSelectableFields(rbr *RoleBindingRestriction) fields.Set {
	return generic.ObjectMetaFieldsSet(&rbr.ObjectMeta, true)
}
