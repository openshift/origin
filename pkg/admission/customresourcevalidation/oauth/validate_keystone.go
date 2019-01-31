package oauth

import (
	"net/url"

	"k8s.io/apimachinery/pkg/util/validation/field"

	configv1 "github.com/openshift/api/config/v1"
)

func ValidateKeystoneIdentityProvider(provider *configv1.KeystoneIdentityProvider, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	if provider == nil {
		errs = append(errs, field.Required(fldPath, ""))
		return errs
	}

	errs = append(errs, ValidateRemoteConnectionInfo(provider.OAuthRemoteConnectionInfo, fldPath)...)

	// URL being valid or empty is checked above, only perform https:// schema check
	providerURL, err := url.Parse(provider.OAuthRemoteConnectionInfo.URL)
	if err == nil {
		if providerURL.Scheme != "https" {
			errs = append(errs, field.Invalid(field.NewPath("url"), provider.OAuthRemoteConnectionInfo.URL, "Auth URL should be secure and start with https"))
		}
	}
	if len(provider.DomainName) == 0 {
		errs = append(errs, field.Required(field.NewPath("domainName"), ""))
	}

	return errs
}
