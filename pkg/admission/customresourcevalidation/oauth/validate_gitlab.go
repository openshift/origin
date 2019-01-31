package oauth

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	configv1 "github.com/openshift/api/config/v1"
	crvalidation "github.com/openshift/origin/pkg/admission/customresourcevalidation"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation/common"
)

func ValidateGitLabIdentityProvider(provider *configv1.GitLabIdentityProvider, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if provider == nil {
		allErrs = append(allErrs, field.Required(fieldPath, ""))
		return allErrs
	}

	allErrs = append(allErrs, ValidateOAuthIdentityProvider(provider.ClientID, provider.ClientSecret, fieldPath)...)

	_, urlErrs := common.ValidateSecureURL(provider.URL, fieldPath.Child("url"))
	allErrs = append(allErrs, urlErrs...)

	allErrs = append(allErrs, crvalidation.ValidateConfigMapReference(fieldPath.Child("ca"), provider.CA, false)...)

	return allErrs
}
