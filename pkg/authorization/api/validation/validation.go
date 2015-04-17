package validation

import (
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func ValidatePolicyName(name string, prefix bool) (bool, string) {
	if name != authorizationapi.PolicyName {
		return false, "name must be " + authorizationapi.PolicyName
	}

	return true, ""
}

func ValidatePolicy(policy *authorizationapi.Policy) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMeta(&policy.ObjectMeta, true, ValidatePolicyName).Prefix("metadata")...)

	for roleKey, role := range policy.Roles {
		if roleKey != role.Name {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("roles."+roleKey+".metadata.name", role.Name, "must be "+roleKey))
		}

		allErrs = append(allErrs, ValidateRole(&role).Prefix("roles."+roleKey)...)
	}

	return allErrs
}

func ValidatePolicyUpdate(policy *authorizationapi.Policy, oldPolicy *authorizationapi.Policy) fielderrors.ValidationErrorList {
	allErrs := ValidatePolicy(policy)
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&oldPolicy.ObjectMeta, &policy.ObjectMeta).Prefix("metadata")...)

	return allErrs
}

func PolicyBindingNameValidator(policyRefNamespace string) validation.ValidateNameFunc {
	return func(name string, prefix bool) (bool, string) {
		if name != policyRefNamespace {
			return false, "name must be " + policyRefNamespace
		}

		return true, ""
	}
}

// ValidatePolicyBinding tests required fields for a PolicyBinding.
func ValidatePolicyBinding(policyBinding *authorizationapi.PolicyBinding) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMeta(&policyBinding.ObjectMeta, true, PolicyBindingNameValidator(policyBinding.PolicyRef.Namespace)).Prefix("metadata")...)

	if len(policyBinding.PolicyRef.Namespace) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("policyRef.namespace"))
	}

	for roleBindingKey, roleBinding := range policyBinding.RoleBindings {
		if roleBinding.RoleRef.Namespace != policyBinding.PolicyRef.Namespace {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("roleBindings."+roleBindingKey+".roleRef.namespace", policyBinding.PolicyRef.Namespace, "must be "+policyBinding.PolicyRef.Namespace))
		}

		if roleBindingKey != roleBinding.Name {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("roleBindings."+roleBindingKey+".metadata.name", roleBinding.Name, "must be "+roleBindingKey))
		}

		allErrs = append(allErrs, ValidateRoleBinding(&roleBinding).Prefix("roleBindings."+roleBindingKey)...)
	}

	return allErrs
}

func ValidatePolicyBindingUpdate(policyBinding *authorizationapi.PolicyBinding, oldPolicyBinding *authorizationapi.PolicyBinding) fielderrors.ValidationErrorList {
	allErrs := ValidatePolicyBinding(policyBinding)
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&oldPolicyBinding.ObjectMeta, &policyBinding.ObjectMeta).Prefix("metadata")...)

	if oldPolicyBinding.PolicyRef.Namespace != policyBinding.PolicyRef.Namespace {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("policyRef.namespace", policyBinding.PolicyRef.Namespace, "cannot change policyRef"))
	}

	return allErrs
}

func ValidateRoleName(name string, prefix bool) (bool, string) {
	if strings.Contains(name, "%") {
		return false, `may not contain "%"`
	}
	if strings.Contains(name, "/") {
		return false, `may not contain "/"`
	}
	return true, ""
}

// ValidateRole tests required fields for a Role.
func ValidateRole(role *authorizationapi.Role) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMeta(&role.ObjectMeta, true, ValidateRoleName).Prefix("metadata")...)

	return allErrs
}

func ValidateRoleUpdate(role *authorizationapi.Role, oldRole *authorizationapi.Role) fielderrors.ValidationErrorList {
	allErrs := ValidateRole(role)
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&oldRole.ObjectMeta, &role.ObjectMeta).Prefix("metadata")...)

	return allErrs
}

func ValidateRoleBindingName(name string, prefix bool) (bool, string) {
	if strings.Contains(name, "%") {
		return false, `may not contain "%"`
	}
	if strings.Contains(name, "/") {
		return false, `may not contain "/"`
	}
	return true, ""
}

// ValidateRoleBinding tests required fields for a RoleBinding.
func ValidateRoleBinding(roleBinding *authorizationapi.RoleBinding) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMeta(&roleBinding.ObjectMeta, true, ValidateRoleBindingName).Prefix("metadata")...)

	if len(roleBinding.RoleRef.Namespace) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("roleRef.namespace"))
	} else if !util.IsDNS1123Subdomain(roleBinding.RoleRef.Namespace) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("roleRef.namespace", roleBinding.RoleRef.Namespace, "roleRef.namespace must be a valid subdomain"))
	}

	if len(roleBinding.RoleRef.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("roleRef.name"))
	} else {
		if valid, err := ValidateRoleName(roleBinding.RoleRef.Name, false); !valid {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("roleRef.name", roleBinding.RoleRef.Name, err))
		}
	}

	return allErrs
}

func ValidateRoleBindingUpdate(roleBinding *authorizationapi.RoleBinding, oldRoleBinding *authorizationapi.RoleBinding) fielderrors.ValidationErrorList {
	allErrs := ValidateRoleBinding(roleBinding)
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&oldRoleBinding.ObjectMeta, &roleBinding.ObjectMeta).Prefix("metadata")...)

	if oldRoleBinding.RoleRef != roleBinding.RoleRef {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("roleRef", roleBinding.RoleRef, "cannot change roleRef"))
	}

	return allErrs
}
