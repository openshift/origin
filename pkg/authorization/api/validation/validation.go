package validation

import (
	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// ValidatePolicyBinding tests required fields for a PolicyBinding.
func ValidatePolicyBinding(policyBinding *authorizationapi.PolicyBinding) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	if len(policyBinding.Name) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("name"))
	}

	if len(policyBinding.PolicyRef.Namespace) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("policyRef.Namespace"))
	}

	allErrs = append(allErrs, validation.ValidateLabels(policyBinding.Labels, "labels")...)
	return allErrs
}

// ValidateRole tests required fields for a Role.
func ValidateRole(role *authorizationapi.Role) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	if len(role.Name) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("name"))
	}

	allErrs = append(allErrs, validation.ValidateLabels(role.Labels, "labels")...)
	return allErrs
}

// ValidateRoleBinding tests required fields for a RoleBinding.
func ValidateRoleBinding(roleBinding *authorizationapi.RoleBinding) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	if len(roleBinding.Name) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("name"))
	}

	if len(roleBinding.RoleRef.Namespace) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("roleRef.namespace"))
	} else if !util.IsDNS1123Subdomain(roleBinding.RoleRef.Namespace) {
		allErrs = append(allErrs, errs.NewFieldInvalid("roleRef.namespace", roleBinding.RoleRef.Namespace, "roleRef.namespace must be a valid subdomain"))
	}

	allErrs = append(allErrs, validation.ValidateLabels(roleBinding.Labels, "labels")...)
	return allErrs
}
