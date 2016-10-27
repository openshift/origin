package validation

import (
	"fmt"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
	kvalidation "k8s.io/kubernetes/pkg/util/validation"
	"k8s.io/kubernetes/pkg/util/validation/field"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	uservalidation "github.com/openshift/origin/pkg/user/api/validation"
)

func ValidateSelfSubjectRulesReview(review *authorizationapi.SelfSubjectRulesReview) field.ErrorList {
	return field.ErrorList{}
}

func ValidateSubjectRulesReview(rules *authorizationapi.SubjectRulesReview) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(rules.Spec.Groups) == 0 && len(rules.Spec.User) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("user"), "at least one of user and groups must be specified"))
	}

	return allErrs
}

func ValidateSubjectAccessReview(review *authorizationapi.SubjectAccessReview) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(review.Action.Verb) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("verb"), ""))
	}
	if len(review.Action.Resource) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("resource"), ""))
	}

	return allErrs
}

func ValidateResourceAccessReview(review *authorizationapi.ResourceAccessReview) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(review.Action.Verb) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("verb"), ""))
	}
	if len(review.Action.Resource) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("resource"), ""))
	}

	return allErrs
}

func ValidateLocalSubjectAccessReview(review *authorizationapi.LocalSubjectAccessReview) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(review.Action.Verb) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("verb"), ""))
	}
	if len(review.Action.Resource) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("resource"), ""))
	}

	return allErrs
}

func ValidateLocalResourceAccessReview(review *authorizationapi.LocalResourceAccessReview) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(review.Action.Verb) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("verb"), ""))
	}
	if len(review.Action.Resource) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("resource"), ""))
	}

	return allErrs
}

func ValidatePolicyName(name string, prefix bool) []string {
	if reasons := oapi.MinimalNameRequirements(name, prefix); len(reasons) != 0 {
		return reasons
	}

	if name != authorizationapi.PolicyName {
		return []string{"name must be " + authorizationapi.PolicyName}
	}

	return nil
}

func ValidateLocalPolicy(policy *authorizationapi.Policy) field.ErrorList {
	return ValidatePolicy(policy, true)
}

func ValidateLocalPolicyUpdate(policy *authorizationapi.Policy, oldPolicy *authorizationapi.Policy) field.ErrorList {
	return ValidatePolicyUpdate(policy, oldPolicy, true)
}

func ValidateClusterPolicy(policy *authorizationapi.ClusterPolicy) field.ErrorList {
	return ValidatePolicy(authorizationapi.ToPolicy(policy), false)
}

func ValidateClusterPolicyUpdate(policy *authorizationapi.ClusterPolicy, oldPolicy *authorizationapi.ClusterPolicy) field.ErrorList {
	return ValidatePolicyUpdate(authorizationapi.ToPolicy(policy), authorizationapi.ToPolicy(oldPolicy), false)
}

func ValidatePolicy(policy *authorizationapi.Policy, isNamespaced bool) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&policy.ObjectMeta, isNamespaced, ValidatePolicyName, field.NewPath("metadata"))

	rolePath := field.NewPath("roles")
	for roleKey, role := range policy.Roles {
		keyPath := rolePath.Key(roleKey)
		if role == nil {
			allErrs = append(allErrs, field.Required(keyPath, ""))
		}

		if roleKey != role.Name {
			allErrs = append(allErrs, field.Invalid(keyPath.Child("metadata", "name"), role.Name, "must be "+roleKey))
		}

		allErrs = append(allErrs, validateRole(role, isNamespaced, keyPath)...)
	}

	return allErrs
}

func ValidatePolicyUpdate(policy *authorizationapi.Policy, oldPolicy *authorizationapi.Policy, isNamespaced bool) field.ErrorList {
	allErrs := ValidatePolicy(policy, isNamespaced)
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&policy.ObjectMeta, &oldPolicy.ObjectMeta, field.NewPath("metadata"))...)

	return allErrs
}

func PolicyBindingNameValidator(policyRefNamespace string) validation.ValidateNameFunc {
	return func(name string, prefix bool) []string {
		if reasons := oapi.MinimalNameRequirements(name, prefix); len(reasons) != 0 {
			return reasons
		}

		if name != authorizationapi.GetPolicyBindingName(policyRefNamespace) {
			return []string{"name must be " + authorizationapi.GetPolicyBindingName(policyRefNamespace)}
		}

		return nil
	}
}

func ValidateLocalPolicyBinding(policy *authorizationapi.PolicyBinding) field.ErrorList {
	return ValidatePolicyBinding(policy, true)
}

func ValidateLocalPolicyBindingUpdate(policy *authorizationapi.PolicyBinding, oldPolicyBinding *authorizationapi.PolicyBinding) field.ErrorList {
	return ValidatePolicyBindingUpdate(policy, oldPolicyBinding, true)
}

func ValidateClusterPolicyBinding(policy *authorizationapi.ClusterPolicyBinding) field.ErrorList {
	return ValidatePolicyBinding(authorizationapi.ToPolicyBinding(policy), false)
}

func ValidateClusterPolicyBindingUpdate(policy *authorizationapi.ClusterPolicyBinding, oldPolicyBinding *authorizationapi.ClusterPolicyBinding) field.ErrorList {
	return ValidatePolicyBindingUpdate(authorizationapi.ToPolicyBinding(policy), authorizationapi.ToPolicyBinding(oldPolicyBinding), false)
}

func ValidatePolicyBinding(policyBinding *authorizationapi.PolicyBinding, isNamespaced bool) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&policyBinding.ObjectMeta, isNamespaced, PolicyBindingNameValidator(policyBinding.PolicyRef.Namespace), field.NewPath("metadata"))

	if !isNamespaced {
		if len(policyBinding.PolicyRef.Namespace) > 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("policyRef", "namespace"), policyBinding.PolicyRef.Namespace, "may not reference another namespace"))
		}
	}

	roleBindingsPath := field.NewPath("roleBindings")
	for roleBindingKey, roleBinding := range policyBinding.RoleBindings {
		keyPath := roleBindingsPath.Key(roleBindingKey)
		if roleBinding == nil {
			allErrs = append(allErrs, field.Required(keyPath, ""))
		}

		if roleBinding.RoleRef.Namespace != policyBinding.PolicyRef.Namespace {
			allErrs = append(allErrs, field.Invalid(keyPath.Child("roleRef", "namespace"), policyBinding.PolicyRef.Namespace, "must be "+policyBinding.PolicyRef.Namespace))
		}

		if roleBindingKey != roleBinding.Name {
			allErrs = append(allErrs, field.Invalid(keyPath.Child("metadata", "name"), roleBinding.Name, "must be "+roleBindingKey))
		}

		allErrs = append(allErrs, validateRoleBinding(roleBinding, isNamespaced, keyPath)...)
	}

	return allErrs
}

func ValidatePolicyBindingUpdate(policyBinding *authorizationapi.PolicyBinding, oldPolicyBinding *authorizationapi.PolicyBinding, isNamespaced bool) field.ErrorList {
	allErrs := ValidatePolicyBinding(policyBinding, isNamespaced)
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&policyBinding.ObjectMeta, &oldPolicyBinding.ObjectMeta, field.NewPath("metadata"))...)

	if oldPolicyBinding.PolicyRef.Namespace != policyBinding.PolicyRef.Namespace {
		allErrs = append(allErrs, field.Invalid(field.NewPath("policyRef", "namespace"), policyBinding.PolicyRef.Namespace, "cannot change policyRef"))
	}

	return allErrs
}

func ValidateLocalRole(policy *authorizationapi.Role) field.ErrorList {
	return ValidateRole(policy, true)
}

func ValidateLocalRoleUpdate(policy *authorizationapi.Role, oldRole *authorizationapi.Role) field.ErrorList {
	return ValidateRoleUpdate(policy, oldRole, true)
}

func ValidateClusterRole(policy *authorizationapi.ClusterRole) field.ErrorList {
	return ValidateRole(authorizationapi.ToRole(policy), false)
}

func ValidateClusterRoleUpdate(policy *authorizationapi.ClusterRole, oldRole *authorizationapi.ClusterRole) field.ErrorList {
	return ValidateRoleUpdate(authorizationapi.ToRole(policy), authorizationapi.ToRole(oldRole), false)
}

func ValidateRole(role *authorizationapi.Role, isNamespaced bool) field.ErrorList {
	return validateRole(role, isNamespaced, nil)
}

func validateRole(role *authorizationapi.Role, isNamespaced bool, fldPath *field.Path) field.ErrorList {
	return validation.ValidateObjectMeta(&role.ObjectMeta, isNamespaced, oapi.MinimalNameRequirements, fldPath.Child("metadata"))
}

func ValidateRoleUpdate(role *authorizationapi.Role, oldRole *authorizationapi.Role, isNamespaced bool) field.ErrorList {
	allErrs := ValidateRole(role, isNamespaced)
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&role.ObjectMeta, &oldRole.ObjectMeta, field.NewPath("metadata"))...)

	return allErrs
}

func ValidateLocalRoleBinding(policy *authorizationapi.RoleBinding) field.ErrorList {
	return ValidateRoleBinding(policy, true)
}

func ValidateLocalRoleBindingUpdate(policy *authorizationapi.RoleBinding, oldRoleBinding *authorizationapi.RoleBinding) field.ErrorList {
	return ValidateRoleBindingUpdate(policy, oldRoleBinding, true)
}

func ValidateClusterRoleBinding(policy *authorizationapi.ClusterRoleBinding) field.ErrorList {
	return ValidateRoleBinding(authorizationapi.ToRoleBinding(policy), false)
}

func ValidateClusterRoleBindingUpdate(policy *authorizationapi.ClusterRoleBinding, oldRoleBinding *authorizationapi.ClusterRoleBinding) field.ErrorList {
	return ValidateRoleBindingUpdate(authorizationapi.ToRoleBinding(policy), authorizationapi.ToRoleBinding(oldRoleBinding), false)
}

func ValidateRoleBinding(roleBinding *authorizationapi.RoleBinding, isNamespaced bool) field.ErrorList {
	return validateRoleBinding(roleBinding, isNamespaced, nil)
}

func validateRoleBinding(roleBinding *authorizationapi.RoleBinding, isNamespaced bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMeta(&roleBinding.ObjectMeta, isNamespaced, oapi.MinimalNameRequirements, fldPath.Child("metadata"))...)

	// roleRef namespace is empty when referring to global policy.
	if (len(roleBinding.RoleRef.Namespace) > 0) && len(kvalidation.IsDNS1123Subdomain(roleBinding.RoleRef.Namespace)) != 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("roleRef", "namespace"), roleBinding.RoleRef.Namespace, "roleRef.namespace must be a valid subdomain"))
	}

	if len(roleBinding.RoleRef.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("roleRef", "name"), ""))
	} else {
		if reasons := oapi.MinimalNameRequirements(roleBinding.RoleRef.Name, false); len(reasons) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("roleRef", "name"), roleBinding.RoleRef.Name, strings.Join(reasons, ", ")))
		}
	}

	subjectsPath := field.NewPath("subjects")
	for i, subject := range roleBinding.Subjects {
		allErrs = append(allErrs, validateRoleBindingSubject(subject, isNamespaced, subjectsPath.Index(i))...)
	}

	return allErrs
}

func validateRoleBindingSubject(subject kapi.ObjectReference, isNamespaced bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(subject.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
	}
	if len(subject.UID) != 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("uid"), fmt.Sprintf("%v", subject.UID)))
	}
	if len(subject.APIVersion) != 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("apiVersion"), subject.APIVersion))
	}
	if len(subject.ResourceVersion) != 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("resourceVersion"), subject.ResourceVersion))
	}
	if len(subject.FieldPath) != 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("fieldPath"), subject.FieldPath))
	}

	switch subject.Kind {
	case authorizationapi.ServiceAccountKind:
		if reasons := validation.ValidateServiceAccountName(subject.Name, false); len(subject.Name) > 0 && len(reasons) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), subject.Name, strings.Join(reasons, ", ")))
		}
		if !isNamespaced && len(subject.Namespace) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), "Service account subjects for ClusterRoleBindings must have a namespace"))
		}

	case authorizationapi.UserKind:
		if reasons := uservalidation.ValidateUserName(subject.Name, false); len(subject.Name) > 0 && len(reasons) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), subject.Name, strings.Join(reasons, ", ")))
		}

	case authorizationapi.GroupKind:
		if reasons := uservalidation.ValidateGroupName(subject.Name, false); len(subject.Name) > 0 && len(reasons) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), subject.Name, strings.Join(reasons, ", ")))
		}

	case authorizationapi.SystemUserKind:
		isValidSAName := len(validation.ValidateServiceAccountName(subject.Name, false)) == 0
		isValidUserName := len(uservalidation.ValidateUserName(subject.Name, false)) == 0
		if isValidSAName || isValidUserName {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), subject.Name, "conforms to User.name or ServiceAccount.name restrictions"))
		}

	case authorizationapi.SystemGroupKind:
		if reasons := uservalidation.ValidateGroupName(subject.Name, false); len(subject.Name) > 0 && len(reasons) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), subject.Name, "conforms to Group.name restrictions"))
		}

	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("kind"), subject.Kind, []string{authorizationapi.ServiceAccountKind, authorizationapi.UserKind, authorizationapi.GroupKind, authorizationapi.SystemGroupKind, authorizationapi.SystemUserKind}))
	}

	return allErrs
}

func ValidateRoleBindingUpdate(roleBinding *authorizationapi.RoleBinding, oldRoleBinding *authorizationapi.RoleBinding, isNamespaced bool) field.ErrorList {
	allErrs := ValidateRoleBinding(roleBinding, isNamespaced)
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&roleBinding.ObjectMeta, &oldRoleBinding.ObjectMeta, field.NewPath("metadata"))...)

	if oldRoleBinding.RoleRef != roleBinding.RoleRef {
		allErrs = append(allErrs, field.Invalid(field.NewPath("roleRef"), roleBinding.RoleRef, "cannot change roleRef"))
	}

	return allErrs
}
