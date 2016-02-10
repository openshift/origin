package validation

import (
	"fmt"
	"strings"

	kvalidation "k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util/validation/field"

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

func ValidateGroup(group *api.Group) field.ErrorList {
	allErrs := kvalidation.ValidateObjectMeta(&group.ObjectMeta, false, ValidateGroupName, field.NewPath("metadata"))

	userPath := field.NewPath("user")
	for index, user := range group.Users {
		idxPath := userPath.Index(index)
		if len(user) == 0 {
			allErrs = append(allErrs, field.Invalid(idxPath, user, "may not be empty"))
			continue
		}
		if ok, msg := ValidateUserName(user, false); !ok {
			allErrs = append(allErrs, field.Invalid(idxPath, user, msg))
		}
	}

	return allErrs
}

func ValidateGroupUpdate(group *api.Group, old *api.Group) field.ErrorList {
	allErrs := kvalidation.ValidateObjectMetaUpdate(&group.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateGroup(group)...)
	return allErrs
}

func ValidateUser(user *api.User) field.ErrorList {
	allErrs := kvalidation.ValidateObjectMeta(&user.ObjectMeta, false, ValidateUserName, field.NewPath("metadata"))
	identitiesPath := field.NewPath("identities")
	for index, identity := range user.Identities {
		idxPath := identitiesPath.Index(index)
		if ok, msg := ValidateIdentityName(identity, false); !ok {
			allErrs = append(allErrs, field.Invalid(idxPath, identity, msg))
		}
	}

	groupsPath := field.NewPath("groups")
	for index, group := range user.Groups {
		idxPath := groupsPath.Index(index)
		if len(group) == 0 {
			allErrs = append(allErrs, field.Invalid(idxPath, group, "may not be empty"))
			continue
		}
		if ok, msg := ValidateGroupName(group, false); !ok {
			allErrs = append(allErrs, field.Invalid(idxPath, group, msg))
		}
	}

	return allErrs
}

func ValidateUserUpdate(user *api.User, old *api.User) field.ErrorList {
	allErrs := kvalidation.ValidateObjectMetaUpdate(&user.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateUser(user)...)
	return allErrs
}

func ValidateIdentity(identity *api.Identity) field.ErrorList {
	allErrs := kvalidation.ValidateObjectMeta(&identity.ObjectMeta, false, ValidateIdentityName, field.NewPath("metadata"))

	if len(identity.ProviderName) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("providerName"), ""))
	} else if ok, msg := ValidateIdentityProviderName(identity.ProviderName); !ok {
		allErrs = append(allErrs, field.Invalid(field.NewPath("providerName"), identity.ProviderName, msg))
	}

	if len(identity.ProviderUserName) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("providerUserName"), ""))
	} else if ok, msg := ValidateIdentityProviderName(identity.ProviderUserName); !ok {
		allErrs = append(allErrs, field.Invalid(field.NewPath("providerUserName"), identity.ProviderUserName, msg))
	}

	userPath := field.NewPath("user")
	if len(identity.ProviderName) > 0 && len(identity.ProviderUserName) > 0 {
		expectedIdentityName := identity.ProviderName + ":" + identity.ProviderUserName
		if identity.Name != expectedIdentityName {
			allErrs = append(allErrs, field.Invalid(userPath.Child("name"), identity.User.Name, fmt.Sprintf("must be %s", expectedIdentityName)))
		}
	}

	if ok, msg := ValidateUserName(identity.User.Name, false); !ok {
		allErrs = append(allErrs, field.Invalid(userPath.Child("name"), identity.User.Name, msg))
	}
	if len(identity.User.Name) == 0 && len(identity.User.UID) != 0 {
		allErrs = append(allErrs, field.Invalid(userPath.Child("uid"), identity.User.UID, "may not be set if user.name is empty"))
	}
	if len(identity.User.Name) != 0 && len(identity.User.UID) == 0 {
		allErrs = append(allErrs, field.Required(userPath.Child("uid"), ""))
	}
	return allErrs
}

func ValidateIdentityUpdate(identity *api.Identity, old *api.Identity) field.ErrorList {
	allErrs := kvalidation.ValidateObjectMetaUpdate(&identity.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateIdentity(identity)...)

	if identity.ProviderName != old.ProviderName {
		allErrs = append(allErrs, field.Invalid(field.NewPath("providerName"), identity.ProviderName, "may not change providerName"))
	}
	if identity.ProviderUserName != old.ProviderUserName {
		allErrs = append(allErrs, field.Invalid(field.NewPath("providerUserName"), identity.ProviderUserName, "may not change providerUserName"))
	}

	return allErrs
}

func ValidateUserIdentityMapping(mapping *api.UserIdentityMapping) field.ErrorList {
	allErrs := kvalidation.ValidateObjectMeta(&mapping.ObjectMeta, false, ValidateIdentityName, field.NewPath("metadata"))

	identityPath := field.NewPath("identity")
	if len(mapping.Identity.Name) == 0 {
		allErrs = append(allErrs, field.Required(identityPath.Child("name"), ""))
	}
	if mapping.Identity.Name != mapping.Name {
		allErrs = append(allErrs, field.Invalid(identityPath.Child("name"), mapping.Identity.Name, "must match metadata.name"))
	}
	if len(mapping.User.Name) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("user", "name"), ""))
	}
	return allErrs
}

func ValidateUserIdentityMappingUpdate(mapping *api.UserIdentityMapping, old *api.UserIdentityMapping) field.ErrorList {
	allErrs := kvalidation.ValidateObjectMetaUpdate(&mapping.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateUserIdentityMapping(mapping)...)
	return allErrs
}
