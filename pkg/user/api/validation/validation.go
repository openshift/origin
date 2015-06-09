package validation

import (
	"fmt"
	"strings"

	kvalidation "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/user/api"
)

func ValidateUserName(name string, _ bool) (bool, string) {
	if ok, reason := oapi.MinimalNameRequirements(name, false); !ok {
		return ok, reason
	}

	if strings.Contains(name, ":") {
		return false, `may not contain ":"`
	}
	if name == "~" {
		return false, `may not equal "~"`
	}
	return true, ""
}

func ValidateIdentityName(name string, _ bool) (bool, string) {
	if ok, reason := oapi.MinimalNameRequirements(name, false); !ok {
		return ok, reason
	}

	parts := strings.Split(name, ":")
	if len(parts) != 2 {
		return false, `must be in the format <providerName>:<providerUserName>`
	}
	if len(parts[0]) == 0 {
		return false, `must be in the format <providerName>:<providerUserName> with a non-empty providerName`
	}
	if len(parts[1]) == 0 {
		return false, `must be in the format <providerName>:<providerUserName> with a non-empty providerUserName`
	}
	return true, ""
}

func ValidateGroupName(name string, _ bool) (bool, string) {
	if ok, reason := oapi.MinimalNameRequirements(name, false); !ok {
		return ok, reason
	}

	if strings.Contains(name, ":") {
		return false, `may not contain ":"`
	}
	if name == "~" {
		return false, `may not equal "~"`
	}
	return true, ""
}

func ValidateIdentityProviderName(name string) (bool, string) {
	if ok, reason := oapi.MinimalNameRequirements(name, false); !ok {
		return ok, reason
	}

	if strings.Contains(name, ":") {
		return false, `may not contain ":"`
	}
	return true, ""
}

func ValidateIdentityProviderUserName(name string) (bool, string) {
	// Any provider user name must be a valid user name
	return ValidateUserName(name, false)
}

func ValidateUser(user *api.User) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, kvalidation.ValidateObjectMeta(&user.ObjectMeta, false, ValidateUserName).Prefix("metadata")...)
	for index, identity := range user.Identities {
		if ok, msg := ValidateIdentityName(identity, false); !ok {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid(fmt.Sprintf("identities[%d]", index), identity, msg))
		}
	}

	for index, group := range user.Groups {
		if len(group) == 0 {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid(fmt.Sprintf("groups[%d]", index), group, "may not be empty"))
			continue
		}
		if ok, msg := ValidateGroupName(group, false); !ok {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid(fmt.Sprintf("groups[%d]", index), group, msg))
		}
	}

	return allErrs
}

func ValidateUserUpdate(user *api.User, old *api.User) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, kvalidation.ValidateObjectMetaUpdate(&old.ObjectMeta, &user.ObjectMeta).Prefix("metadata")...)
	allErrs = append(allErrs, ValidateUser(user)...)
	return allErrs
}

func ValidateIdentity(identity *api.Identity) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, kvalidation.ValidateObjectMeta(&identity.ObjectMeta, false, ValidateIdentityName).Prefix("metadata")...)

	if len(identity.ProviderName) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("providerName"))
	} else if ok, msg := ValidateIdentityProviderName(identity.ProviderName); !ok {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("providerName", identity.ProviderName, msg))
	}

	if len(identity.ProviderUserName) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("providerUserName"))
	} else if ok, msg := ValidateIdentityProviderName(identity.ProviderUserName); !ok {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("providerUserName", identity.ProviderUserName, msg))
	}

	if len(identity.ProviderName) > 0 && len(identity.ProviderUserName) > 0 {
		expectedIdentityName := identity.ProviderName + ":" + identity.ProviderUserName
		if identity.Name != expectedIdentityName {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("user.name", identity.User.Name, fmt.Sprintf("must be %s", expectedIdentityName)))
		}
	}

	if ok, msg := ValidateUserName(identity.User.Name, false); !ok {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("user.name", identity.User.Name, msg))
	}
	if len(identity.User.Name) == 0 && len(identity.User.UID) != 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("user.uid", identity.User.UID, "may not be set if user.name is empty"))
	}
	if len(identity.User.Name) != 0 && len(identity.User.UID) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("user.uid"))
	}
	return allErrs
}

func ValidateIdentityUpdate(identity *api.Identity, old *api.Identity) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, kvalidation.ValidateObjectMetaUpdate(&old.ObjectMeta, &identity.ObjectMeta).Prefix("metadata")...)
	allErrs = append(allErrs, ValidateIdentity(identity)...)

	if identity.ProviderName != old.ProviderName {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("providerName", identity.ProviderName, "may not change providerName"))
	}
	if identity.ProviderUserName != old.ProviderUserName {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("providerUserName", identity.ProviderUserName, "may not change providerUserName"))
	}

	return allErrs
}

func ValidateUserIdentityMapping(mapping *api.UserIdentityMapping) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, kvalidation.ValidateObjectMeta(&mapping.ObjectMeta, false, ValidateIdentityName).Prefix("metadata")...)
	if len(mapping.Identity.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("identity.name"))
	}
	if mapping.Identity.Name != mapping.Name {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("identity.name", mapping.Identity.Name, "must match metadata.name"))
	}
	if len(mapping.User.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("user.name"))
	}
	return allErrs
}

func ValidateUserIdentityMappingUpdate(mapping *api.UserIdentityMapping, old *api.UserIdentityMapping) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, kvalidation.ValidateObjectMetaUpdate(&old.ObjectMeta, &mapping.ObjectMeta).Prefix("metadata")...)
	allErrs = append(allErrs, ValidateUserIdentityMapping(mapping)...)
	return allErrs
}
