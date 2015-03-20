package validation

import (
	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/openshift/origin/pkg/oauth/api"
)

func ValidateAccessToken(accessToken *api.OAuthAccessToken) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(accessToken.Name) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("name"))
	}
	if len(accessToken.ClientName) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("clientname"))
	}
	if len(accessToken.UserName) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("username"))
	}
	if len(accessToken.UserUID) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("useruid"))
	}
	if len(accessToken.Namespace) != 0 {
		allErrs = append(allErrs, errs.NewFieldInvalid("namespace", accessToken.Namespace, "namespace must be empty"))
	}
	allErrs = append(allErrs, validateLabels(accessToken.Labels)...)
	return allErrs
}

func ValidateAuthorizeToken(authorizeToken *api.OAuthAuthorizeToken) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(authorizeToken.Name) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("name"))
	}
	if len(authorizeToken.ClientName) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("clientname"))
	}
	if len(authorizeToken.UserName) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("username"))
	}
	if len(authorizeToken.UserUID) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("useruid"))
	}
	if len(authorizeToken.Namespace) != 0 {
		allErrs = append(allErrs, errs.NewFieldInvalid("namespace", authorizeToken.Namespace, "namespace must be empty"))
	}
	allErrs = append(allErrs, validateLabels(authorizeToken.Labels)...)
	return allErrs
}

func ValidateClient(client *api.OAuthClient) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(client.Name) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("name"))
	}
	if len(client.Namespace) != 0 {
		allErrs = append(allErrs, errs.NewFieldInvalid("namespace", client.Namespace, "namespace must be empty"))
	}
	allErrs = append(allErrs, validateLabels(client.Labels)...)
	return allErrs
}

func ValidateClientAuthorization(clientAuthorization *api.OAuthClientAuthorization) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(clientAuthorization.Name) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("name"))
	}
	if len(clientAuthorization.ClientName) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("clientname"))
	}
	if len(clientAuthorization.UserName) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("username"))
	}
	if len(clientAuthorization.UserUID) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("useruid"))
	}
	if len(clientAuthorization.Namespace) != 0 {
		allErrs = append(allErrs, errs.NewFieldInvalid("namespace", clientAuthorization.Namespace, "namespace must be empty"))
	}
	allErrs = append(allErrs, validateLabels(clientAuthorization.Labels)...)
	return allErrs
}

func ValidateClientAuthorizationUpdate(newAuth *api.OAuthClientAuthorization, oldAuth *api.OAuthClientAuthorization) errs.ValidationErrorList {
	allErrs := ValidateClientAuthorization(newAuth)
	if oldAuth.Name != newAuth.Name {
		allErrs = append(allErrs, errs.NewFieldInvalid("name", newAuth.Name, "name is not a mutable field"))
	}
	if oldAuth.ClientName != newAuth.ClientName {
		allErrs = append(allErrs, errs.NewFieldInvalid("clientname", newAuth.ClientName, "clientname is not a mutable field"))
	}
	if oldAuth.UserName != newAuth.UserName {
		allErrs = append(allErrs, errs.NewFieldInvalid("username", newAuth.UserName, "username is not a mutable field"))
	}
	if oldAuth.UserUID != newAuth.UserUID {
		allErrs = append(allErrs, errs.NewFieldInvalid("useruid", newAuth.UserUID, "useruid is not a mutable field"))
	}
	allErrs = append(allErrs, validateLabels(newAuth.Labels)...)
	return allErrs
}

func validateLabels(labels map[string]string) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	for k := range labels {
		if !util.IsDNS952Label(k) {
			allErrs = append(allErrs, errs.NewFieldNotSupported("label", k))
		}
	}
	return allErrs
}
