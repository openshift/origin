package configconversion

import (
	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
)

func GetOAuthConfigFileReferences(config *osinv1.OAuthConfig) []*string {
	if config == nil {
		return []*string{}
	}

	refs := []*string{}

	if config.MasterCA != nil {
		refs = append(refs, config.MasterCA)
	}

	refs = append(refs, GetSessionConfigFileReferences(config.SessionConfig)...)
	for _, identityProvider := range config.IdentityProviders {
		switch provider := identityProvider.Provider.Object.(type) {
		case (*osinv1.RequestHeaderIdentityProvider):
			refs = append(refs, &provider.ClientCA)

		case (*osinv1.HTPasswdPasswordIdentityProvider):
			refs = append(refs, &provider.File)

		case (*osinv1.LDAPPasswordIdentityProvider):
			refs = append(refs, &provider.CA)
			refs = append(refs, helpers.GetStringSourceFileReferences(&provider.BindPassword)...)

		case (*osinv1.BasicAuthPasswordIdentityProvider):
			refs = append(refs, helpers.GetRemoteConnectionInfoFileReferences(&provider.RemoteConnectionInfo)...)

		case (*osinv1.KeystonePasswordIdentityProvider):
			refs = append(refs, helpers.GetRemoteConnectionInfoFileReferences(&provider.RemoteConnectionInfo)...)

		case (*osinv1.GitLabIdentityProvider):
			refs = append(refs, &provider.CA)
			refs = append(refs, helpers.GetStringSourceFileReferences(&provider.ClientSecret)...)

		case (*osinv1.OpenIDIdentityProvider):
			refs = append(refs, &provider.CA)
			refs = append(refs, helpers.GetStringSourceFileReferences(&provider.ClientSecret)...)

		case (*osinv1.GoogleIdentityProvider):
			refs = append(refs, helpers.GetStringSourceFileReferences(&provider.ClientSecret)...)

		case (*osinv1.GitHubIdentityProvider):
			refs = append(refs, helpers.GetStringSourceFileReferences(&provider.ClientSecret)...)
			refs = append(refs, &provider.CA)

		}
	}

	if config.Templates != nil {
		refs = append(refs, &config.Templates.Login)
		refs = append(refs, &config.Templates.ProviderSelection)
		refs = append(refs, &config.Templates.Error)
	}

	return refs
}

func GetSessionConfigFileReferences(config *osinv1.SessionConfig) []*string {
	if config == nil {
		return []*string{}
	}

	refs := []*string{}
	refs = append(refs, &config.SessionSecretsFile)
	return refs
}
