package oauth

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	configv1 "github.com/openshift/api/config/v1"
	crvalidation "github.com/openshift/origin/pkg/admission/customresourcevalidation"
)

func ValidateLDAPIdentityProvider(provider *configv1.LDAPIdentityProvider, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if provider == nil {
		errs = append(errs, field.Required(fldPath, ""))
		return errs
	}

	errs = append(errs, crvalidation.ValidateSecretReference(fldPath.Child("bindPassword"), provider.BindPassword, false)...)

	// At least one attribute to use as the user id is required
	if len(provider.Attributes.ID) == 0 {
		errs = append(errs, field.Invalid(fldPath.Child("attributes", "id"), "[]", "at least one id attribute is required (LDAP standard identity attribute is 'dn')"))
	}

	return errs
}
