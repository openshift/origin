package validation

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// ValidatePolicyBinding tests required fields for a PolicyBinding.
func ValidatePolicyBinding(policyBinding *authorizationapi.PolicyBinding) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(policyBinding.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
	}

	if len(policyBinding.PolicyRef.Namespace) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("policyRef.Namespace"))
	}

	allErrs = append(allErrs, validation.ValidateLabels(policyBinding.Labels, "labels")...)
	return allErrs
}

// ValidateRole tests required fields for a Role.
func ValidateRole(role *authorizationapi.Role) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(role.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
	}

	allErrs = append(allErrs, validation.ValidateLabels(role.Labels, "labels")...)
	return allErrs
}

// ValidateRoleBinding tests required fields for a RoleBinding.
func ValidateRoleBinding(roleBinding *authorizationapi.RoleBinding) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(roleBinding.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
	}

	if len(roleBinding.RoleRef.Namespace) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("roleRef.namespace"))
	} else if !util.IsDNS1123Subdomain(roleBinding.RoleRef.Namespace) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("roleRef.namespace", roleBinding.RoleRef.Namespace, "roleRef.namespace must be a valid subdomain"))
	}

	allErrs = append(allErrs, validation.ValidateLabels(roleBinding.Labels, "labels")...)
	return allErrs
}
