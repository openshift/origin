package validation

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
	"github.com/openshift/origin/pkg/oauth/api"
)

func ValidateAccessToken(accessToken *api.OAuthAccessToken) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if len(accessToken.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
	}
	if len(accessToken.ClientName) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("clientname"))
	}
	if len(accessToken.UserName) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("username"))
	}
	if len(accessToken.UserUID) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("useruid"))
	}
	if len(accessToken.Namespace) != 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("namespace", accessToken.Namespace, "namespace must be empty"))
	}
	allErrs = append(allErrs, validateLabels(accessToken.Labels)...)
	return allErrs
}

func ValidateAuthorizeToken(authorizeToken *api.OAuthAuthorizeToken) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if len(authorizeToken.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
	}
	if len(authorizeToken.ClientName) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("clientname"))
	}
	if len(authorizeToken.UserName) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("username"))
	}
	if len(authorizeToken.UserUID) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("useruid"))
	}
	if len(authorizeToken.Namespace) != 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("namespace", authorizeToken.Namespace, "namespace must be empty"))
	}
	allErrs = append(allErrs, validateLabels(authorizeToken.Labels)...)
	return allErrs
}

func ValidateClient(client *api.OAuthClient) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if len(client.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
	}
	if len(client.Namespace) != 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("namespace", client.Namespace, "namespace must be empty"))
	}
	allErrs = append(allErrs, validateLabels(client.Labels)...)
	return allErrs
}

func ValidateClientAuthorization(clientAuthorization *api.OAuthClientAuthorization) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if len(clientAuthorization.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
	}
	if len(clientAuthorization.ClientName) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("clientname"))
	}
	if len(clientAuthorization.UserName) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("username"))
	}
	if len(clientAuthorization.UserUID) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("useruid"))
	}
	if len(clientAuthorization.Namespace) != 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("namespace", clientAuthorization.Namespace, "namespace must be empty"))
	}
	allErrs = append(allErrs, validateLabels(clientAuthorization.Labels)...)
	return allErrs
}

func ValidateClientAuthorizationUpdate(newAuth *api.OAuthClientAuthorization, oldAuth *api.OAuthClientAuthorization) fielderrors.ValidationErrorList {
	allErrs := ValidateClientAuthorization(newAuth)
	if oldAuth.Name != newAuth.Name {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("name", newAuth.Name, "name is not a mutable field"))
	}
	if oldAuth.ClientName != newAuth.ClientName {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("clientname", newAuth.ClientName, "clientname is not a mutable field"))
	}
	if oldAuth.UserName != newAuth.UserName {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("username", newAuth.UserName, "username is not a mutable field"))
	}
	if oldAuth.UserUID != newAuth.UserUID {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("useruid", newAuth.UserUID, "useruid is not a mutable field"))
	}
	allErrs = append(allErrs, validateLabels(newAuth.Labels)...)
	return allErrs
}

func validateLabels(labels map[string]string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	for k := range labels {
		if !util.IsDNS952Label(k) {
			allErrs = append(allErrs, fielderrors.NewFieldNotSupported("label", k))
		}
	}
	return allErrs
}
