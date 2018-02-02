package validation

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	oauthvalidation "github.com/openshift/origin/pkg/oauth/apis/oauth/validation"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/redirector"
	"github.com/openshift/origin/pkg/oauthserver/server/errorpage"
	"github.com/openshift/origin/pkg/oauthserver/server/login"
	"github.com/openshift/origin/pkg/oauthserver/server/selectprovider"
	"github.com/openshift/origin/pkg/oauthserver/userregistry/identitymapper"
	"github.com/openshift/origin/pkg/user/apis/user/validation"
)

func ValidateOAuthConfig(config *configapi.OAuthConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	if config.MasterCA == nil {
		validationResults.AddErrors(field.Invalid(fldPath.Child("masterCA"), config.MasterCA, "a filename or empty string is required"))
	} else if len(*config.MasterCA) > 0 {
		validationResults.AddErrors(ValidateFile(*config.MasterCA, fldPath.Child("masterCA"))...)
	}

	if len(config.MasterURL) == 0 {
		validationResults.AddErrors(field.Required(fldPath.Child("masterURL"), ""))
	} else if _, urlErrs := ValidateURL(config.MasterURL, fldPath.Child("masterURL")); len(urlErrs) > 0 {
		validationResults.AddErrors(urlErrs...)
	}

	if _, urlErrs := ValidateURL(config.MasterPublicURL, fldPath.Child("masterPublicURL")); len(urlErrs) > 0 {
		validationResults.AddErrors(urlErrs...)
	}

	if len(config.AssetPublicURL) == 0 {
		validationResults.AddErrors(field.Required(fldPath.Child("assetPublicURL"), ""))
	}

	if config.SessionConfig != nil {
		validationResults.AddErrors(validateSessionConfig(config.SessionConfig, fldPath.Child("sessionConfig"))...)
	}

	validationResults.AddErrors(validateGrantConfig(config.GrantConfig, fldPath.Child("grantConfig"))...)

	providerNames := sets.NewString()
	redirectingIdentityProviders := []string{}

	challengeIssuingIdentityProviders := []string{}
	challengeRedirectingIdentityProviders := []string{}

	for i, identityProvider := range config.IdentityProviders {
		if identityProvider.UseAsLogin {
			redirectingIdentityProviders = append(redirectingIdentityProviders, identityProvider.Name)

			if configapi.IsPasswordAuthenticator(identityProvider) {
				if config.SessionConfig == nil {
					validationResults.AddErrors(field.Invalid(fldPath.Child("sessionConfig"), config, "sessionConfig is required if a password identity provider is used for browser based login"))
				}
			}
		}

		if identityProvider.UseAsChallenger {
			// RequestHeaderIdentityProvider is special, it can only react to challenge clients by redirecting them
			// Make sure we don't have more than a single redirector, and don't have a mix of challenge issuers and redirectors
			if _, isRequestHeader := identityProvider.Provider.(*configapi.RequestHeaderIdentityProvider); isRequestHeader {
				challengeRedirectingIdentityProviders = append(challengeRedirectingIdentityProviders, identityProvider.Name)
			} else {
				challengeIssuingIdentityProviders = append(challengeIssuingIdentityProviders, identityProvider.Name)
			}
		}

		identityProviderPath := fldPath.Child("identityProvider").Index(i)
		validationResults.Append(ValidateIdentityProvider(identityProvider, identityProviderPath))

		if len(identityProvider.Name) > 0 {
			if providerNames.Has(identityProvider.Name) {
				validationResults.AddErrors(field.Invalid(identityProviderPath.Child("name"), identityProvider.Name, "must have a unique name"))
			}
			providerNames.Insert(identityProvider.Name)
		}
	}

	if len(redirectingIdentityProviders) == 0 {
		validationResults.AddWarnings(field.Invalid(fldPath.Child("identityProviders"), "login", "no identity providers are configured to handle logins"))
	}
	if len(challengeRedirectingIdentityProviders) > 1 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("identityProviders"), "challenge", fmt.Sprintf("only one identity provider can redirect clients requesting an authentication challenge, found: %v", strings.Join(challengeRedirectingIdentityProviders, ", "))))
	}
	if len(challengeRedirectingIdentityProviders) > 0 && len(challengeIssuingIdentityProviders) > 0 {
		validationResults.AddErrors(
			field.Invalid(fldPath.Child("identityProviders"), "challenge", fmt.Sprintf(
				"cannot mix providers that redirect clients requesting auth challenges (%s) with providers issuing challenges to those clients (%s)",
				strings.Join(challengeRedirectingIdentityProviders, ", "),
				strings.Join(challengeIssuingIdentityProviders, ", "),
			)))
	}

	if timeout := config.TokenConfig.AccessTokenInactivityTimeoutSeconds; timeout != nil {
		if *timeout != 0 && *timeout < oauthvalidation.MinimumInactivityTimeoutSeconds {
			validationResults.AddErrors(field.Invalid(
				fldPath.Child("tokenConfig", "accessTokenInactivityTimeoutSeconds"), *timeout,
				fmt.Sprintf("The minimum acceptable token timeout value is %d seconds",
					oauthvalidation.MinimumInactivityTimeoutSeconds)))
		}
	}

	if config.Templates != nil {
		if len(config.Templates.Login) > 0 {
			content, err := ioutil.ReadFile(config.Templates.Login)
			if err != nil {
				validationResults.AddErrors(field.Invalid(fldPath.Child("templates", "login"), config.Templates.Login, "could not read file"))
			} else {
				for _, err = range login.ValidateLoginTemplate(content) {
					validationResults.AddErrors(field.Invalid(fldPath.Child("templates", "login"), config.Templates.Login, err.Error()))
				}
			}
		}

		if len(config.Templates.ProviderSelection) > 0 {
			content, err := ioutil.ReadFile(config.Templates.ProviderSelection)
			if err != nil {
				validationResults.AddErrors(field.Invalid(fldPath.Child("templates", "providerSelection"), config.Templates.ProviderSelection, "could not read file"))
			} else {
				for _, err = range selectprovider.ValidateSelectProviderTemplate(content) {
					validationResults.AddErrors(field.Invalid(fldPath.Child("templates", "providerSelection"), config.Templates.ProviderSelection, err.Error()))
				}
			}
		}

		if len(config.Templates.Error) > 0 {
			content, err := ioutil.ReadFile(config.Templates.Error)
			if err != nil {
				validationResults.AddErrors(field.Invalid(fldPath.Child("templates", "error"), config.Templates.Error, "could not read file"))
			} else {
				for _, err = range errorpage.ValidateErrorPageTemplate(content) {
					validationResults.AddErrors(field.Invalid(fldPath.Child("templates", "error"), config.Templates.Error, err.Error()))
				}
			}
		}
	}

	return validationResults
}

var validMappingMethods = sets.NewString(
	string(identitymapper.MappingMethodLookup),
	string(identitymapper.MappingMethodClaim),
	string(identitymapper.MappingMethodAdd),
	string(identitymapper.MappingMethodGenerate),
)

func ValidateIdentityProvider(identityProvider configapi.IdentityProvider, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	if len(identityProvider.Name) == 0 {
		validationResults.AddErrors(field.Required(fldPath.Child("name"), ""))
	}
	if reasons := validation.ValidateIdentityProviderName(identityProvider.Name); len(reasons) != 0 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("name"), identityProvider.Name, strings.Join(reasons, ", ")))
	}

	if len(identityProvider.MappingMethod) == 0 {
		validationResults.AddErrors(field.Required(fldPath.Child("mappingMethod"), ""))
	} else if !validMappingMethods.Has(identityProvider.MappingMethod) {
		validationResults.AddErrors(field.NotSupported(fldPath.Child("mappingMethod"), identityProvider.MappingMethod, validMappingMethods.List()))
	}

	providerPath := fldPath.Child("provider")
	if !configapi.IsIdentityProviderType(identityProvider.Provider) {
		validationResults.AddErrors(field.Invalid(fldPath.Child("provider"), identityProvider.Provider, fmt.Sprintf("%v is invalid in this context", identityProvider.Provider)))
	} else {
		switch provider := identityProvider.Provider.(type) {
		case (*configapi.RequestHeaderIdentityProvider):
			validationResults.Append(ValidateRequestHeaderIdentityProvider(provider, identityProvider, fldPath))

		case (*configapi.BasicAuthPasswordIdentityProvider):
			validationResults.AddErrors(ValidateRemoteConnectionInfo(provider.RemoteConnectionInfo, providerPath)...)

		case (*configapi.HTPasswdPasswordIdentityProvider):
			validationResults.AddErrors(ValidateFile(provider.File, providerPath.Child("file"))...)

		case (*configapi.LDAPPasswordIdentityProvider):
			validationResults.Append(ValidateLDAPIdentityProvider(provider, providerPath))

		case (*configapi.KeystonePasswordIdentityProvider):
			validationResults.Append(ValidateKeystoneIdentityProvider(provider, identityProvider, providerPath))

		case (*configapi.GitHubIdentityProvider):
			validationResults.Append(ValidateGitHubIdentityProvider(provider, identityProvider.UseAsChallenger, identityProvider.MappingMethod, fldPath))

		case (*configapi.GitLabIdentityProvider):
			validationResults.AddErrors(ValidateGitLabIdentityProvider(provider, fldPath)...)

		case (*configapi.GoogleIdentityProvider):
			validationResults.Append(ValidateGoogleIdentityProvider(provider, identityProvider.UseAsChallenger, identityProvider.MappingMethod, fldPath))

		case (*configapi.OpenIDIdentityProvider):
			validationResults.AddErrors(ValidateOpenIDIdentityProvider(provider, identityProvider, fldPath)...)

		}
	}

	return validationResults
}

func ValidateLDAPIdentityProvider(provider *configapi.LDAPPasswordIdentityProvider, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.Append(ValidateStringSource(provider.BindPassword, fldPath.Child("bindPassword")))
	bindPassword, _ := configapi.ResolveStringValue(provider.BindPassword)
	validationResults.Append(ValidateLDAPClientConfig(provider.URL, provider.BindDN, bindPassword, provider.CA, provider.Insecure, fldPath))

	// At least one attribute to use as the user id is required
	if len(provider.Attributes.ID) == 0 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("attributes", "id"), "[]", "at least one id attribute is required (LDAP standard identity attribute is 'dn')"))
	}

	return validationResults
}

// RemoteConnection fields validated separately -- this is for keystone-specific validation
func ValidateKeystoneIdentityProvider(provider *configapi.KeystonePasswordIdentityProvider, identityProvider configapi.IdentityProvider, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}
	validationResults.AddErrors(ValidateRemoteConnectionInfo(provider.RemoteConnectionInfo, fldPath)...)

	providerURL, err := url.Parse(provider.RemoteConnectionInfo.URL)
	if err == nil {
		if providerURL.Scheme != "https" {
			validationResults.AddWarnings(field.Invalid(field.NewPath("url"), provider.RemoteConnectionInfo.URL, "Auth URL should be secure and start with https"))
		}
	}
	if len(provider.DomainName) == 0 {
		validationResults.AddErrors(field.Required(field.NewPath("domainName"), ""))
	}

	return validationResults
}

func ValidateRequestHeaderIdentityProvider(provider *configapi.RequestHeaderIdentityProvider, identityProvider configapi.IdentityProvider, fieldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	if len(provider.ClientCA) > 0 {
		validationResults.AddErrors(ValidateFile(provider.ClientCA, fieldPath.Child("provider", "clientCA"))...)
	} else if len(provider.ClientCommonNames) > 0 {
		validationResults.AddErrors(field.Invalid(fieldPath.Child("provider", "clientCommonNames"), provider.ClientCommonNames, "clientCA must be specified in order to use clientCommonNames"))
	}

	if len(provider.Headers) == 0 {
		validationResults.AddErrors(field.Required(fieldPath.Child("provider", "headers"), ""))
	}
	if identityProvider.UseAsChallenger && len(provider.ChallengeURL) == 0 {
		err := field.Required(fieldPath.Child("provider", "challengeURL"), "challengeURL is required if challenge is true")
		validationResults.AddErrors(err)
	}
	if identityProvider.UseAsLogin && len(provider.LoginURL) == 0 {
		err := field.Required(fieldPath.Child("provider", "loginURL"), "loginURL is required if login=true")
		validationResults.AddErrors(err)
	}

	if len(provider.ChallengeURL) > 0 {
		url, urlErrs := ValidateURL(provider.ChallengeURL, fieldPath.Child("provider", "challengeURL"))
		validationResults.AddErrors(urlErrs...)
		if len(urlErrs) == 0 && !strings.Contains(url.RawQuery, redirector.URLToken) && !strings.Contains(url.RawQuery, redirector.QueryToken) {
			validationResults.AddWarnings(
				field.Invalid(
					field.NewPath("provider", "challengeURL"),
					provider.ChallengeURL,
					fmt.Sprintf("query does not include %q or %q, redirect will not preserve original authorize parameters", redirector.URLToken, redirector.QueryToken),
				),
			)
		}
	}
	if len(provider.LoginURL) > 0 {
		url, urlErrs := ValidateURL(provider.LoginURL, fieldPath.Child("provider", "loginURL"))
		validationResults.AddErrors(urlErrs...)
		if len(urlErrs) == 0 {
			if !strings.Contains(url.RawQuery, redirector.URLToken) && !strings.Contains(url.RawQuery, redirector.QueryToken) {
				validationResults.AddWarnings(
					field.Invalid(
						fieldPath.Child("provider", "loginURL"),
						provider.LoginURL,
						fmt.Sprintf("query does not include %q or %q, redirect will not preserve original authorize parameters", redirector.URLToken, redirector.QueryToken),
					),
				)
			}
			if strings.HasSuffix(url.Path, "/") {
				validationResults.AddWarnings(
					field.Invalid(fieldPath.Child("provider", "loginURL"), provider.LoginURL, `path ends with "/", grant approval flows will not function correctly`),
				)
			}
			if _, file := path.Split(url.Path); file != "authorize" {
				validationResults.AddWarnings(
					field.Invalid(fieldPath.Child("provider", "loginURL"), provider.LoginURL, `path does not end with "/authorize", grant approval flows will not function correctly`),
				)
			}
		}
	}

	// Warn if it looks like they expect direct requests to the OAuth endpoints, and have not secured the header checking with a client certificate check
	if len(provider.ClientCA) == 0 && (len(provider.ChallengeURL) > 0 || len(provider.LoginURL) > 0) {
		validationResults.AddWarnings(field.Invalid(fieldPath.Child("provider", "clientCA"), "", "if no clientCA is set, no request verification is done, and any request directly against the OAuth server can impersonate any identity from this provider"))
	}

	return validationResults
}

func ValidateOAuthIdentityProvider(clientID string, clientSecret configapi.StringSource, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(clientID) == 0 {
		allErrs = append(allErrs, field.Required(fieldPath.Child("provider", "clientID"), ""))
	}
	clientSecretResults := ValidateStringSource(clientSecret, fieldPath.Child("provider", "clientSecret"))
	allErrs = append(allErrs, clientSecretResults.Errors...)
	if len(clientSecretResults.Errors) == 0 {
		clientSecret, err := configapi.ResolveStringValue(clientSecret)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fieldPath.Child("provider", "clientSecret"), "", err.Error()))
		} else if len(clientSecret) == 0 {
			allErrs = append(allErrs, field.Required(fieldPath.Child("provider", "clientSecret"), ""))
		}
	}

	return allErrs
}

func ValidateGitHubIdentityProvider(provider *configapi.GitHubIdentityProvider, challenge bool, mappingMethod string, fieldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.AddErrors(ValidateOAuthIdentityProvider(provider.ClientID, provider.ClientSecret, fieldPath)...)

	if challenge {
		validationResults.AddErrors(field.Invalid(fieldPath.Child("challenge"), challenge, "A GitHub identity provider cannot be used for challenges"))
	}

	if len(provider.Teams) > 0 && len(provider.Organizations) > 0 {
		validationResults.AddErrors(field.Invalid(fieldPath.Child("organizations"), provider.Organizations, "specify organizations or teams, not both"))
		validationResults.AddErrors(field.Invalid(fieldPath.Child("teams"), provider.Teams, "specify organizations or teams, not both"))
	}
	if len(provider.Teams) == 0 && len(provider.Organizations) == 0 && mappingMethod != string(identitymapper.MappingMethodLookup) {
		validationResults.AddWarnings(field.Invalid(fieldPath, nil, "no organizations or teams specified, any GitHub user will be allowed to authenticate"))
	}
	for i, team := range provider.Teams {
		if len(strings.Split(team, "/")) != 2 {
			validationResults.AddErrors(field.Invalid(fieldPath.Child("teams").Index(i), team, "must be in the format <org>/<team>"))
		}
	}

	return validationResults
}

func ValidateGoogleIdentityProvider(provider *configapi.GoogleIdentityProvider, challenge bool, mappingMethod string, fieldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.AddErrors(ValidateOAuthIdentityProvider(provider.ClientID, provider.ClientSecret, fieldPath)...)

	if challenge {
		validationResults.AddErrors(field.Invalid(fieldPath.Child("challenge"), challenge, "A Google identity provider cannot be used for challenges"))
	}

	if len(provider.HostedDomain) == 0 && mappingMethod != string(identitymapper.MappingMethodLookup) {
		validationResults.AddWarnings(field.Invalid(fieldPath, nil, "no hostedDomain specified, any Google user will be allowed to authenticate"))
	}

	return validationResults
}

func ValidateGitLabIdentityProvider(provider *configapi.GitLabIdentityProvider, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateOAuthIdentityProvider(provider.ClientID, provider.ClientSecret, fieldPath)...)

	_, urlErrs := ValidateSecureURL(provider.URL, fieldPath.Child("provider", "url"))
	allErrs = append(allErrs, urlErrs...)

	if len(provider.CA) != 0 {
		allErrs = append(allErrs, ValidateFile(provider.CA, fieldPath.Child("provider", "ca"))...)
	}

	return allErrs
}

func ValidateOpenIDIdentityProvider(provider *configapi.OpenIDIdentityProvider, identityProvider configapi.IdentityProvider, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateOAuthIdentityProvider(provider.ClientID, provider.ClientSecret, fieldPath)...)

	// Communication with the Authorization Endpoint MUST utilize TLS
	// http://openid.net/specs/openid-connect-core-1_0.html#AuthorizationEndpoint
	providerPath := fieldPath.Child("provider")
	urlsPath := providerPath.Child("urls")
	_, urlErrs := ValidateSecureURL(provider.URLs.Authorize, urlsPath.Child("authorize"))
	allErrs = append(allErrs, urlErrs...)

	// Communication with the Token Endpoint MUST utilize TLS
	// http://openid.net/specs/openid-connect-core-1_0.html#TokenEndpoint
	_, urlErrs = ValidateSecureURL(provider.URLs.Token, urlsPath.Child("token"))
	allErrs = append(allErrs, urlErrs...)

	if len(provider.URLs.UserInfo) != 0 {
		// Communication with the UserInfo Endpoint MUST utilize TLS
		// http://openid.net/specs/openid-connect-core-1_0.html#UserInfo
		_, urlErrs = ValidateSecureURL(provider.URLs.UserInfo, urlsPath.Child("userInfo"))
		allErrs = append(allErrs, urlErrs...)
	}

	// At least one claim to use as the user id is required
	if len(provider.Claims.ID) == 0 {
		allErrs = append(allErrs, field.Invalid(providerPath.Child("claims", "id"), "[]", "at least one id claim is required (OpenID standard identity claim is 'sub')"))
	}

	if len(provider.CA) != 0 {
		allErrs = append(allErrs, ValidateFile(provider.CA, providerPath.Child("ca"))...)
	}

	return allErrs
}

func validateGrantConfig(config configapi.GrantConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if !configapi.ValidGrantHandlerTypes.Has(string(config.Method)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("method"), config.Method, configapi.ValidGrantHandlerTypes.List()))
	}
	if !configapi.ValidServiceAccountGrantHandlerTypes.Has(string(config.ServiceAccountMethod)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("serviceAccountMethod"), config.ServiceAccountMethod, configapi.ValidServiceAccountGrantHandlerTypes.List()))
	}

	return allErrs
}

func validateSessionConfig(config *configapi.SessionConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate session secrets file, if specified
	sessionSecretsFilePath := fldPath.Child("sessionSecretsFile")
	if len(config.SessionSecretsFile) > 0 {
		fileErrs := ValidateFile(config.SessionSecretsFile, sessionSecretsFilePath)
		if len(fileErrs) != 0 {
			// Missing file
			allErrs = append(allErrs, fileErrs...)
		} else {
			// Validate file contents
			secrets, err := latest.ReadSessionSecrets(config.SessionSecretsFile)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(sessionSecretsFilePath, config.SessionSecretsFile, fmt.Sprintf("error reading file: %v", err)))
			} else {
				for _, err := range ValidateSessionSecrets(secrets) {
					allErrs = append(allErrs, field.Invalid(sessionSecretsFilePath, config.SessionSecretsFile, err.Error()))
				}
			}
		}
	}

	if len(config.SessionName) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("sessionName"), ""))
	}

	return allErrs
}

func ValidateSessionSecrets(config *configapi.SessionSecrets) field.ErrorList {
	allErrs := field.ErrorList{}

	secretsPath := field.NewPath("secrets")
	if len(config.Secrets) == 0 {
		allErrs = append(allErrs, field.Required(secretsPath, ""))
	}

	for i, secret := range config.Secrets {
		idxPath := secretsPath.Index(i)
		switch {
		case len(secret.Authentication) == 0:
			allErrs = append(allErrs, field.Required(idxPath.Child("authentication"), ""))
		case len(secret.Authentication) < 32:
			// Don't output current value in error message... we don't want it logged
			allErrs = append(allErrs,
				field.Invalid(
					idxPath.Child("authentication"),
					strings.Repeat("*", len(secret.Authentication)),
					"must be at least 32 characters long",
				),
			)
		}

		switch len(secret.Encryption) {
		case 0:
			// Require encryption secrets
			allErrs = append(allErrs, field.Required(idxPath.Child("encryption"), ""))
		case 16, 24, 32:
			// Valid lengths
		default:
			// Don't output current value in error message... we don't want it logged
			allErrs = append(allErrs,
				field.Invalid(
					idxPath.Child("encryption"),
					strings.Repeat("*", len(secret.Encryption)),
					"must be 16, 24, or 32 characters long",
				),
			)
		}
	}

	return allErrs
}
