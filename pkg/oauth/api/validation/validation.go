package validation

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/oauth/api"
	uservalidation "github.com/openshift/origin/pkg/user/api/validation"
)

const MinTokenLength = 32

func ValidateTokenName(name string, prefix bool) (bool, string) {
	if ok, reason := oapi.MinimalNameRequirements(name, prefix); !ok {
		return ok, reason
	}

	if len(name) < MinTokenLength {
		return false, fmt.Sprintf("must be at least %d characters long", MinTokenLength)
	}
	return true, ""
}

func ValidateAccessToken(accessToken *api.OAuthAccessToken) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, validation.ValidateObjectMeta(&accessToken.ObjectMeta, false, ValidateTokenName).Prefix("metadata")...)
	allErrs = append(allErrs, ValidateClientNameField(accessToken.ClientName, "clientName")...)
	allErrs = append(allErrs, ValidateUserNameField(accessToken.UserName, "userName")...)

	if len(accessToken.UserUID) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("userUID"))
	}

	return allErrs
}

func ValidateAuthorizeToken(authorizeToken *api.OAuthAuthorizeToken) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, validation.ValidateObjectMeta(&authorizeToken.ObjectMeta, false, ValidateTokenName).Prefix("metadata")...)
	allErrs = append(allErrs, ValidateClientNameField(authorizeToken.ClientName, "clientName")...)
	allErrs = append(allErrs, ValidateUserNameField(authorizeToken.UserName, "userName")...)

	if len(authorizeToken.UserUID) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("userUID"))
	}

	return allErrs
}

func ValidateClient(client *api.OAuthClient) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, validation.ValidateObjectMeta(&client.ObjectMeta, false, validation.NameIsDNSSubdomain).Prefix("metadata")...)

	return allErrs
}

func ValidateClientUpdate(client *api.OAuthClient, oldClient *api.OAuthClient) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateClient(client)...)
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&oldClient.ObjectMeta, &client.ObjectMeta).Prefix("metadata")...)

	return allErrs
}

func ValidateClientAuthorizationName(name string, prefix bool) (bool, string) {
	if ok, reason := oapi.MinimalNameRequirements(name, prefix); !ok {
		return ok, reason
	}

	parts := strings.Split(name, ":")
	if len(parts) != 2 {
		return false, "must be in the format <userName>:<clientName>"
	}

	userName := parts[0]
	clientName := parts[1]
	if len(userName) == 0 || len(clientName) == 0 {
		return false, "must be in the format <userName>:<clientName>"
	}

	return true, ""
}

func ValidateClientAuthorization(clientAuthorization *api.OAuthClientAuthorization) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	expectedName := fmt.Sprintf("%s:%s", clientAuthorization.UserName, clientAuthorization.ClientName)

	metadataErrs := validation.ValidateObjectMeta(&clientAuthorization.ObjectMeta, false, ValidateClientAuthorizationName).Prefix("metadata")
	if len(metadataErrs) > 0 {
		allErrs = append(allErrs, metadataErrs...)
	} else if clientAuthorization.Name != expectedName {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("metadata.name", clientAuthorization.Name, "must be in the format <userName>:<clientName>"))
	}

	allErrs = append(allErrs, ValidateClientNameField(clientAuthorization.ClientName, "clientName")...)
	allErrs = append(allErrs, ValidateUserNameField(clientAuthorization.UserName, "userName")...)

	if len(clientAuthorization.UserUID) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("useruid"))
	}

	return allErrs
}

func ValidateClientAuthorizationUpdate(newAuth *api.OAuthClientAuthorization, oldAuth *api.OAuthClientAuthorization) fielderrors.ValidationErrorList {
	allErrs := ValidateClientAuthorization(newAuth)

	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&oldAuth.ObjectMeta, &newAuth.ObjectMeta).Prefix("metadata")...)

	if oldAuth.ClientName != newAuth.ClientName {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("clientName", newAuth.ClientName, "clientName is not a mutable field"))
	}
	if oldAuth.UserName != newAuth.UserName {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("userName", newAuth.UserName, "userName is not a mutable field"))
	}
	if oldAuth.UserUID != newAuth.UserUID {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("userUID", newAuth.UserUID, "userUID is not a mutable field"))
	}

	return allErrs
}

func ValidateClientNameField(value string, field string) fielderrors.ValidationErrorList {
	if len(value) == 0 {
		return fielderrors.ValidationErrorList{fielderrors.NewFieldRequired(field)}
	} else if ok, msg := validation.NameIsDNSSubdomain(value, false); !ok {
		return fielderrors.ValidationErrorList{fielderrors.NewFieldInvalid(field, value, msg)}
	}
	return fielderrors.ValidationErrorList{}
}

func ValidateUserNameField(value string, field string) fielderrors.ValidationErrorList {
	if len(value) == 0 {
		return fielderrors.ValidationErrorList{fielderrors.NewFieldRequired(field)}
	} else if ok, msg := uservalidation.ValidateUserName(value, false); !ok {
		return fielderrors.ValidationErrorList{fielderrors.NewFieldInvalid(field, value, msg)}
	}
	return fielderrors.ValidationErrorList{}
}
