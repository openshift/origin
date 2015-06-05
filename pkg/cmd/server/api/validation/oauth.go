package validation

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
	"github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/user/api/validation"
)

func ValidateOAuthConfig(config *api.OAuthConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.MasterURL) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("masterURL"))
	}

	if _, urlErrs := ValidateURL(config.MasterPublicURL, "masterPublicURL"); len(urlErrs) > 0 {
		allErrs = append(allErrs, urlErrs...)
	}

	if len(config.AssetPublicURL) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("assetPublicURL"))
	}

	if config.SessionConfig != nil {
		allErrs = append(allErrs, ValidateSessionConfig(config.SessionConfig).Prefix("sessionConfig")...)
	}

	allErrs = append(allErrs, ValidateGrantConfig(config.GrantConfig).Prefix("grantConfig")...)

	providerNames := util.NewStringSet()
	redirectingIdentityProviders := []string{}
	for i, identityProvider := range config.IdentityProviders {
		if identityProvider.UseAsLogin {
			redirectingIdentityProviders = append(redirectingIdentityProviders, identityProvider.Name)

			if api.IsPasswordAuthenticator(identityProvider) {
				if config.SessionConfig == nil {
					allErrs = append(allErrs, fielderrors.NewFieldInvalid("sessionConfig", config, "sessionConfig is required if a password identity provider is used for browser based login"))
				}
			}
		}

		allErrs = append(allErrs, ValidateIdentityProvider(identityProvider).Prefix(fmt.Sprintf("identityProvider[%d]", i))...)

		if len(identityProvider.Name) > 0 {
			if providerNames.Has(identityProvider.Name) {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid(fmt.Sprintf("identityProvider[%d].name", i), identityProvider.Name, "must have a unique name"))
			}
			providerNames.Insert(identityProvider.Name)
		}
	}

	if len(redirectingIdentityProviders) > 1 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("identityProviders", config.IdentityProviders, fmt.Sprintf("only one identity provider can support login for a browser, found: %v", redirectingIdentityProviders)))
	}

	return allErrs
}

func ValidateIdentityProvider(identityProvider api.IdentityProvider) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(identityProvider.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
	}
	if ok, err := validation.ValidateIdentityProviderName(identityProvider.Name); !ok {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("name", identityProvider.Name, err))
	}

	if !api.IsIdentityProviderType(identityProvider.Provider) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("provider", identityProvider.Provider, fmt.Sprintf("%v is invalid in this context", identityProvider.Provider)))
	} else {
		switch provider := identityProvider.Provider.(type) {
		case (*api.RequestHeaderIdentityProvider):
			allErrs = append(allErrs, ValidateRequestHeaderIdentityProvider(provider, identityProvider)...)

		case (*api.BasicAuthPasswordIdentityProvider):
			allErrs = append(allErrs, ValidateRemoteConnectionInfo(provider.RemoteConnectionInfo).Prefix("provider")...)

		case (*api.HTPasswdPasswordIdentityProvider):
			allErrs = append(allErrs, ValidateFile(provider.File, "provider.file")...)

		case (*api.GitHubIdentityProvider):
			allErrs = append(allErrs, ValidateOAuthIdentityProvider(provider.ClientID, provider.ClientSecret, identityProvider.UseAsChallenger)...)

		case (*api.GoogleIdentityProvider):
			allErrs = append(allErrs, ValidateOAuthIdentityProvider(provider.ClientID, provider.ClientSecret, identityProvider.UseAsChallenger)...)

		case (*api.OpenIDIdentityProvider):
			allErrs = append(allErrs, ValidateOpenIDIdentityProvider(provider, identityProvider)...)

		}
	}

	return allErrs
}

func ValidateRequestHeaderIdentityProvider(provider *api.RequestHeaderIdentityProvider, identityProvider api.IdentityProvider) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(provider.ClientCA) > 0 {
		allErrs = append(allErrs, ValidateFile(provider.ClientCA, "provider.clientCA")...)
	}
	if len(provider.Headers) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("provider.headers"))
	}
	if identityProvider.UseAsChallenger {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("challenge", identityProvider.UseAsChallenger, "request header providers cannot be used for challenges"))
	}
	if identityProvider.UseAsLogin {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("login", identityProvider.UseAsChallenger, "request header providers cannot be used for browser login"))
	}

	return allErrs
}

func ValidateOAuthIdentityProvider(clientID, clientSecret string, challenge bool) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(clientID) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("provider.clientID"))
	}
	if len(clientSecret) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("provider.clientSecret"))
	}
	if challenge {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("challenge", challenge, "oauth providers cannot be used for challenges"))
	}

	return allErrs
}

func ValidateOpenIDIdentityProvider(provider *api.OpenIDIdentityProvider, identityProvider api.IdentityProvider) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateOAuthIdentityProvider(provider.ClientID, provider.ClientSecret, identityProvider.UseAsChallenger)...)

	// Communication with the Authorization Endpoint MUST utilize TLS
	// http://openid.net/specs/openid-connect-core-1_0.html#AuthorizationEndpoint
	_, urlErrs := ValidateSecureURL(provider.URLs.Authorize, "authorize")
	allErrs = append(allErrs, urlErrs.Prefix("provider.urls")...)

	// Communication with the Token Endpoint MUST utilize TLS
	// http://openid.net/specs/openid-connect-core-1_0.html#TokenEndpoint
	_, urlErrs = ValidateSecureURL(provider.URLs.Token, "token")
	allErrs = append(allErrs, urlErrs.Prefix("provider.urls")...)

	if len(provider.URLs.UserInfo) != 0 {
		// Communication with the UserInfo Endpoint MUST utilize TLS
		// http://openid.net/specs/openid-connect-core-1_0.html#UserInfo
		_, urlErrs = ValidateSecureURL(provider.URLs.UserInfo, "userInfo")
		allErrs = append(allErrs, urlErrs.Prefix("provider.urls")...)
	}

	// At least one claim to use as the user id is required
	if len(provider.Claims.ID) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("provider.claims.id", "[]", "at least one id claim is required (OpenID standard identity claim is 'sub')"))
	}

	if len(provider.CA) != 0 {
		allErrs = append(allErrs, ValidateFile(provider.CA, "provider.ca")...)
	}

	return allErrs
}

func ValidateGrantConfig(config api.GrantConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if !api.ValidGrantHandlerTypes.Has(string(config.Method)) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("grantConfig.method", config.Method, fmt.Sprintf("must be one of: %v", api.ValidGrantHandlerTypes.List())))
	}

	return allErrs
}

func ValidateSessionConfig(config *api.SessionConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	// Validate session secrets file, if specified
	if len(config.SessionSecretsFile) > 0 {
		fileErrs := ValidateFile(config.SessionSecretsFile, "sessionSecretsFile")
		if len(fileErrs) != 0 {
			// Missing file
			allErrs = append(allErrs, fileErrs...)
		} else {
			// Validate file contents
			secrets, err := latest.ReadSessionSecrets(config.SessionSecretsFile)
			if err != nil {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("sessionSecretsFile", config.SessionSecretsFile, fmt.Sprintf("error reading file: %v", err)))
			} else {
				for _, err := range ValidateSessionSecrets(secrets) {
					allErrs = append(allErrs, fielderrors.NewFieldInvalid("sessionSecretsFile", config.SessionSecretsFile, err.Error()))
				}
			}
		}
	}

	if len(config.SessionName) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("sessionName"))
	}

	return allErrs
}

func ValidateSessionSecrets(config *api.SessionSecrets) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.Secrets) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("secrets"))
	}

	for i, secret := range config.Secrets {
		switch {
		case len(secret.Authentication) == 0:
			allErrs = append(allErrs, fielderrors.NewFieldRequired(fmt.Sprintf("secrets[%d].authentication", i)))
		case len(secret.Authentication) < 32:
			// Don't output current value in error message... we don't want it logged
			allErrs = append(allErrs,
				fielderrors.NewFieldInvalid(
					fmt.Sprintf("secrets[%d].authentpsecretsication", i),
					strings.Repeat("*", len(secret.Authentication)),
					"must be at least 32 characters long",
				),
			)
		}

		switch len(secret.Encryption) {
		case 0:
			// Require encryption secrets
			allErrs = append(allErrs, fielderrors.NewFieldRequired(fmt.Sprintf("secrets[%d].encryption", i)))
		case 16, 24, 32:
			// Valid lengths
		default:
			// Don't output current value in error message... we don't want it logged
			allErrs = append(allErrs,
				fielderrors.NewFieldInvalid(
					fmt.Sprintf("secrets[%d].encryption", i),
					strings.Repeat("*", len(secret.Encryption)),
					"must be 16, 24, or 32 characters long",
				),
			)
		}
	}

	return allErrs
}
