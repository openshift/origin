package validation

import (
	"strings"

	kvalidation "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
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

func ValidateUser(user *api.User) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, kvalidation.ValidateObjectMeta(&user.ObjectMeta, false, ValidateUserName).Prefix("metadata")...)
	return allErrs
}

func ValidateIdentity(identity *api.Identity) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	return allErrs
}

func ValidateUserIdentityMapping(mapping *api.UserIdentityMapping) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, kvalidation.ValidateObjectMeta(&mapping.ObjectMeta, false, ValidateUserName).Prefix("metadata")...)
	allErrs = append(allErrs, ValidateIdentity(&mapping.Identity).Prefix("identity")...)
	allErrs = append(allErrs, ValidateUser(&mapping.User).Prefix("user")...)
	return allErrs
}
