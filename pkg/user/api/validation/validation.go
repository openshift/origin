package validation

import (
	"strings"

	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kvalidation "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/openshift/origin/pkg/user/api"
)

func ValidateUserName(name string, prefix bool) (bool, string) {
	if strings.Contains(name, "%") {
		return false, `Usernames may not contain "%"`
	}
	if strings.Contains(name, "/") {
		return false, `Usernames may not contain "/"`
	}
	if name == ".." {
		return false, `Usernames may not equal ".."`
	}
	if name == "." {
		return false, `Usernames may not equal "."`
	}
	return true, ""
}

func ValidateUser(user *api.User) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	allErrs = append(allErrs, kvalidation.ValidateObjectMeta(&user.ObjectMeta, false, ValidateUserName).Prefix("metadata")...)
	return allErrs
}

func ValidateIdentity(identity *api.Identity) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	return allErrs
}

func ValidateUserIdentityMapping(mapping *api.UserIdentityMapping) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	allErrs = append(allErrs, kvalidation.ValidateObjectMeta(&mapping.ObjectMeta, false, ValidateUserName).Prefix("metadata")...)
	allErrs = append(allErrs, ValidateIdentity(&mapping.Identity).Prefix("identity")...)
	allErrs = append(allErrs, ValidateUser(&mapping.User).Prefix("user")...)
	return allErrs
}
