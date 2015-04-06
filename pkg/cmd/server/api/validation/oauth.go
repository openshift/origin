package validation

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
	"github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/user/api/validation"
)

func ValidateOAuthConfig(config *api.OAuthConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.MasterURL) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("masterURL"))
	}

	if len(config.MasterPublicURL) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("masterPublicURL"))
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
		switch provider := identityProvider.Provider.Object.(type) {
		case (*api.RequestHeaderIdentityProvider):
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

		case (*api.BasicAuthPasswordIdentityProvider):
			allErrs = append(allErrs, ValidateRemoteConnectionInfo(provider.RemoteConnectionInfo).Prefix("provider")...)

		case (*api.HTPasswdPasswordIdentityProvider):
			allErrs = append(allErrs, ValidateFile(provider.File, "provider.file")...)

		case (*api.OAuthRedirectingIdentityProvider):
			if len(provider.ClientID) == 0 {
				allErrs = append(allErrs, fielderrors.NewFieldRequired("provider.clientID"))
			}
			if len(provider.ClientSecret) == 0 {
				allErrs = append(allErrs, fielderrors.NewFieldRequired("provider.clientSecret"))
			}
			if !api.IsOAuthProviderType(provider.Provider) {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("provider.provider", provider.Provider, fmt.Sprintf("%v is invalid in this context", identityProvider.Provider)))
			}
			if identityProvider.UseAsChallenger {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("challenge", identityProvider.UseAsChallenger, "oauth providers cannot be used for challenges"))
			}
		}

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

	if len(config.SessionSecrets) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("sessionSecrets"))
	}
	if len(config.SessionName) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("sessionName"))
	}

	return allErrs
}
