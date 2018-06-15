package validation

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/validation/path"
	unversionedvalidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/core/validation"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	uservalidation "github.com/openshift/origin/pkg/user/apis/user/validation"
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

func validateCommonAccessReviewAction(fldPath *field.Path, action *authorizationapi.Action) field.ErrorList {
	var allErrs field.ErrorList
	if action.IsNonResourceURL {
		if len(action.Path) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("path"), ""))
		}
		if len(action.Resource) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("resource"), action.Resource, "resource may not be specified with non resource URLs"))
		}
		if len(action.Group) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("group"), action.Group, "group may not be specified with non resource URLs"))
		}
		if len(action.Version) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("version"), action.Version, "version may not be specified with non resource URLs"))
		}
		if len(action.ResourceName) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("resourceName"), action.ResourceName, "resourceName may not be specified with non resource URLs"))
		}
		if len(action.Namespace) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), action.Namespace, "namespace may not be specified with non resource URLs"))
		}
		if action.Content != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("content"), nil, "content may not be specified with non resource URLs"))
		}
	} else {
		if len(action.Resource) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("resource"), ""))
		}
	}
	return allErrs
}

func ValidateSubjectAccessReview(review *authorizationapi.SubjectAccessReview) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(review.Action.Verb) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("verb"), ""))
	}
	allErrs = append(allErrs, validateCommonAccessReviewAction(nil, &review.Action)...)

	return allErrs
}

func ValidateResourceAccessReview(review *authorizationapi.ResourceAccessReview) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(review.Action.Verb) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("verb"), ""))
	}
	allErrs = append(allErrs, validateCommonAccessReviewAction(nil, &review.Action)...)

	return allErrs
}

func ValidateLocalSubjectAccessReview(review *authorizationapi.LocalSubjectAccessReview) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(review.Action.Verb) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("verb"), ""))
	}
	allErrs = append(allErrs, validateCommonAccessReviewAction(nil, &review.Action)...)

	return allErrs
}

func ValidateLocalResourceAccessReview(review *authorizationapi.LocalResourceAccessReview) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(review.Action.Verb) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("verb"), ""))
	}
	allErrs = append(allErrs, validateCommonAccessReviewAction(nil, &review.Action)...)

	return allErrs
}

func ValidateLocalRole(policy *authorizationapi.Role) field.ErrorList {
	return ValidateRole(policy, true)
}

func ValidateLocalRoleUpdate(policy *authorizationapi.Role, oldRole *authorizationapi.Role) field.ErrorList {
	return ValidateRoleUpdate(policy, oldRole, true, nil)
}

func ValidateClusterRole(policy *authorizationapi.ClusterRole) field.ErrorList {
	return ValidateRole(authorizationapi.ToRole(policy), false)
}

func ValidateClusterRoleUpdate(policy *authorizationapi.ClusterRole, oldRole *authorizationapi.ClusterRole) field.ErrorList {
	return ValidateRoleUpdate(authorizationapi.ToRole(policy), authorizationapi.ToRole(oldRole), false, nil)
}

func ValidateRole(role *authorizationapi.Role, isNamespaced bool) field.ErrorList {
	return validateRole(role, isNamespaced, nil)
}

func validateRole(role *authorizationapi.Role, isNamespaced bool, fldPath *field.Path) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&role.ObjectMeta, isNamespaced, path.ValidatePathSegmentName, fldPath.Child("metadata"))
	rulesPath := fldPath.Child("rules")
	for i, rule := range role.Rules {
		if rule.AttributeRestrictions != nil {
			allErrs = append(allErrs, field.Invalid(rulesPath.Index(i).Child("attributeRestrictions"), rule.AttributeRestrictions, "must be null"))
		}
	}
	return allErrs
}

func ValidateRoleUpdate(role *authorizationapi.Role, oldRole *authorizationapi.Role, isNamespaced bool, fldPath *field.Path) field.ErrorList {
	allErrs := validateRoleUpdate(role, oldRole, isNamespaced, fldPath)
	// We can use ValidateObjectMetaUpdate here because we know that we are validating a single role, and not a role embedded inside a policy object
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&role.ObjectMeta, &oldRole.ObjectMeta, fldPath.Child("metadata"))...)
	return allErrs
}

func validateRoleUpdate(role *authorizationapi.Role, oldRole *authorizationapi.Role, isNamespaced bool, fldPath *field.Path) field.ErrorList {
	// We use ValidateObjectMeta here because roles embedded inside of policy objects are not guaranteed to
	// have a resource version and thus will fail the policy's validation if ValidateObjectMetaUpdate was used
	allErrs := validation.ValidateObjectMeta(&role.ObjectMeta, isNamespaced, path.ValidatePathSegmentName, fldPath.Child("metadata"))
	rulesPath := fldPath.Child("rules")
	for i, rule := range role.Rules {
		if rule.AttributeRestrictions != nil && isNewRule(rule, oldRole) {
			allErrs = append(allErrs, field.Invalid(rulesPath.Index(i).Child("attributeRestrictions"), rule.AttributeRestrictions, "must be null"))
		}
	}
	return allErrs
}

func isNewRule(rule authorizationapi.PolicyRule, oldRole *authorizationapi.Role) bool {
	for _, r := range oldRole.Rules {
		if r.AttributeRestrictions != nil && equality.Semantic.DeepEqual(rule, r) { // only do expensive comparision against rules that have attribute restrictions
			return false
		}
	}
	return true
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
	allErrs = append(allErrs, validation.ValidateObjectMeta(&roleBinding.ObjectMeta, isNamespaced, path.ValidatePathSegmentName, fldPath.Child("metadata"))...)

	// roleRef namespace is empty when referring to global policy.
	if (len(roleBinding.RoleRef.Namespace) > 0) && len(kvalidation.IsDNS1123Subdomain(roleBinding.RoleRef.Namespace)) != 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("roleRef", "namespace"), roleBinding.RoleRef.Namespace, "roleRef.namespace must be a valid subdomain"))
	}

	if len(roleBinding.RoleRef.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("roleRef", "name"), ""))
	} else {
		if reasons := path.ValidatePathSegmentName(roleBinding.RoleRef.Name, false); len(reasons) != 0 {
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

func ValidateRoleBindingRestriction(rbr *authorizationapi.RoleBindingRestriction) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&rbr.ObjectMeta, true,
		validation.NameIsDNSSubdomain, field.NewPath("metadata"))

	allErrs = append(allErrs,
		ValidateRoleBindingRestrictionSpec(&rbr.Spec, field.NewPath("spec"))...)

	return allErrs
}

func ValidateRoleBindingRestrictionUpdate(rbr, old *authorizationapi.RoleBindingRestriction) field.ErrorList {
	allErrs := ValidateRoleBindingRestriction(rbr)

	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&rbr.ObjectMeta,
		&old.ObjectMeta, field.NewPath("metadata"))...)

	return allErrs
}

func ValidateRoleBindingRestrictionSpec(spec *authorizationapi.RoleBindingRestrictionSpec, fld *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	const invalidMsg = `must specify exactly one of userrestriction, grouprestriction, or serviceaccountrestriction`

	if spec.UserRestriction != nil {
		if spec.GroupRestriction != nil {
			allErrs = append(allErrs, field.Invalid(fld.Child("grouprestriction"),
				"both userrestriction and grouprestriction specified", invalidMsg))
		}
		if spec.ServiceAccountRestriction != nil {
			allErrs = append(allErrs,
				field.Invalid(fld.Child("serviceaccountrestriction"),
					"both userrestriction and serviceaccountrestriction specified", invalidMsg))
		}
	} else if spec.GroupRestriction != nil {
		if spec.ServiceAccountRestriction != nil {
			allErrs = append(allErrs,
				field.Invalid(fld.Child("serviceaccountrestriction"),
					"both grouprestriction and serviceaccountrestriction specified", invalidMsg))
		}
	} else if spec.ServiceAccountRestriction == nil {
		allErrs = append(allErrs, field.Required(fld.Child("userrestriction"),
			invalidMsg))
	}

	if spec.UserRestriction != nil {
		allErrs = append(allErrs, ValidateRoleBindingRestrictionUser(spec.UserRestriction, fld.Child("userrestriction"))...)
	}
	if spec.GroupRestriction != nil {
		allErrs = append(allErrs, ValidateRoleBindingRestrictionGroup(spec.GroupRestriction, fld.Child("grouprestriction"))...)
	}
	if spec.ServiceAccountRestriction != nil {
		allErrs = append(allErrs, ValidateRoleBindingRestrictionServiceAccount(spec.ServiceAccountRestriction, fld.Child("serviceaccountrestriction"))...)
	}

	return allErrs
}

func ValidateRoleBindingRestrictionUser(user *authorizationapi.UserRestriction, fld *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	const invalidMsg = `must specify at least one user, group, or label selector`

	if !(len(user.Users) > 0 || len(user.Groups) > 0 || len(user.Selectors) > 0) {
		allErrs = append(allErrs, field.Required(fld.Child("users"), invalidMsg))
	}

	for i, selector := range user.Selectors {
		allErrs = append(allErrs,
			unversionedvalidation.ValidateLabelSelector(&selector,
				fld.Child("selector").Index(i))...)
	}

	return allErrs
}

func ValidateRoleBindingRestrictionGroup(group *authorizationapi.GroupRestriction, fld *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	const invalidMsg = `must specify at least one group or label selector`

	if !(len(group.Groups) > 0 || len(group.Selectors) > 0) {
		allErrs = append(allErrs, field.Required(fld.Child("groups"), invalidMsg))
	}

	for i, selector := range group.Selectors {
		allErrs = append(allErrs,
			unversionedvalidation.ValidateLabelSelector(&selector,
				fld.Child("selector").Index(i))...)
	}

	return allErrs
}

func ValidateRoleBindingRestrictionServiceAccount(sa *authorizationapi.ServiceAccountRestriction, fld *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	const invalidMsg = `must specify at least one service account or namespace`

	if !(len(sa.ServiceAccounts) > 0 || len(sa.Namespaces) > 0) {
		allErrs = append(allErrs,
			field.Required(fld.Child("serviceaccounts"), invalidMsg))
	}

	return allErrs
}
