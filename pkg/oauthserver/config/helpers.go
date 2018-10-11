package config

import (
	osinv1 "github.com/openshift/api/osin/v1"
)

func IsOAuthIdentityProvider(provider osinv1.IdentityProvider) bool {
	switch provider.Provider.Object.(type) {
	case
		*osinv1.OpenIDIdentityProvider,
		*osinv1.GitHubIdentityProvider,
		*osinv1.GitLabIdentityProvider,
		*osinv1.GoogleIdentityProvider:

		return true
	}

	return false
}

func IsPasswordAuthenticator(provider osinv1.IdentityProvider) bool {
	switch provider.Provider.Object.(type) {
	case
		*osinv1.BasicAuthPasswordIdentityProvider,
		*osinv1.AllowAllPasswordIdentityProvider,
		*osinv1.DenyAllPasswordIdentityProvider,
		*osinv1.HTPasswdPasswordIdentityProvider,
		*osinv1.LDAPPasswordIdentityProvider,
		*osinv1.KeystonePasswordIdentityProvider:

		return true
	}

	return false
}
