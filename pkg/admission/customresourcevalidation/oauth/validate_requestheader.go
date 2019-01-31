package oauth

import (
	"fmt"
	"path"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	configv1 "github.com/openshift/api/config/v1"
	crvalidation "github.com/openshift/origin/pkg/admission/customresourcevalidation"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation/common"
)

const (
	// URLToken in the query of the redirectURL gets replaced with the original request URL, escaped as a query parameter.
	// Example use: https://www.example.com/login?then=${url}
	urlToken = "${url}"

	// ServerRelativeURLToken in the query of the redirectURL gets replaced with the server-relative portion of the original request URL, escaped as a query parameter.
	// Example use: https://www.example.com/login?then=${server-relative-url}
	serverRelativeURLToken = "${server-relative-url}"

	// QueryToken in the query of the redirectURL gets replaced with the original request URL, unescaped.
	// Example use: https://www.example.com/sso/oauth/authorize?${query}
	queryToken = "${query}"
)

func ValidateRequestHeaderIdentityProvider(provider *configv1.RequestHeaderIdentityProvider, fieldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	if provider == nil {
		errs = append(errs, field.Required(fieldPath, ""))
		return errs
	}

	if len(provider.ClientCA.Name) > 0 {
		errs = append(errs, crvalidation.ValidateConfigMapReference(fieldPath.Child("clientCA"), provider.ClientCA, true)...)
	} else if len(provider.ClientCommonNames) > 0 {
		errs = append(errs, field.Invalid(fieldPath.Child("clientCommonNames"), provider.ClientCommonNames, "clientCA must be specified in order to use clientCommonNames"))
	}

	if len(provider.Headers) == 0 {
		errs = append(errs, field.Required(fieldPath.Child("headers"), ""))
	}

	if len(provider.ChallengeURL) > 0 {
		url, urlErrs := common.ValidateURL(provider.ChallengeURL, fieldPath.Child("challengeURL"))
		errs = append(errs, urlErrs...)
		if len(urlErrs) == 0 && !strings.Contains(url.RawQuery, urlToken) && !strings.Contains(url.RawQuery, queryToken) {
			errs = append(errs,
				field.Invalid(
					field.NewPath("challengeURL"),
					provider.ChallengeURL,
					fmt.Sprintf("query does not include %q or %q, redirect will not preserve original authorize parameters", urlToken, queryToken),
				),
			)
		}
	}
	if len(provider.LoginURL) > 0 {
		url, urlErrs := common.ValidateURL(provider.LoginURL, fieldPath.Child("loginURL"))
		errs = append(errs, urlErrs...)
		if len(urlErrs) == 0 {
			if !strings.Contains(url.RawQuery, urlToken) && !strings.Contains(url.RawQuery, queryToken) {
				errs = append(errs,
					field.Invalid(
						fieldPath.Child("loginURL"),
						provider.LoginURL,
						fmt.Sprintf("query does not include %q or %q, redirect will not preserve original authorize parameters", urlToken, queryToken),
					),
				)
			}
			if strings.HasSuffix(url.Path, "/") {
				errs = append(errs,
					field.Invalid(fieldPath.Child("loginURL"), provider.LoginURL, `path ends with "/", grant approval flows will not function correctly`),
				)
			}
			if _, file := path.Split(url.Path); file != "authorize" {
				errs = append(errs,
					field.Invalid(fieldPath.Child("loginURL"), provider.LoginURL, `path does not end with "/authorize", grant approval flows will not function correctly`),
				)
			}
		}
	}

	// Warn if it looks like they expect direct requests to the OAuth endpoints, and have not secured the header checking with a client certificate check
	if len(provider.ClientCA.Name) == 0 && (len(provider.ChallengeURL) > 0 || len(provider.LoginURL) > 0) {
		errs = append(errs, field.Invalid(fieldPath.Child("clientCA"), "", "if no clientCA is set, no request verification is done, and any request directly against the OAuth server can impersonate any identity from this provider"))
	}

	return errs
}
