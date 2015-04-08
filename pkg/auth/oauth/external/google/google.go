package google

import (
	"github.com/openshift/origin/pkg/auth/oauth/external"
	"github.com/openshift/origin/pkg/auth/oauth/external/openid"
)

const (
	googleAuthorizeURL = "https://accounts.google.com/o/oauth2/auth"
	googleTokenURL     = "https://www.googleapis.com/oauth2/v3/token"
	googleUserInfoURL  = "https://www.googleapis.com/oauth2/v3/userinfo"
)

var googleOAuthScopes = []string{"openid", "email", "profile"}

type provider struct {
	providerName, clientID, clientSecret string
}

func NewProvider(providerName, clientID, clientSecret string) (external.Provider, error) {
	config := openid.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,

		AuthorizeURL: googleAuthorizeURL,
		TokenURL:     googleTokenURL,
		UserInfoURL:  googleUserInfoURL,

		Scopes: googleOAuthScopes,

		IDClaims:                []string{"sub"},
		PreferredUsernameClaims: []string{"email"},
		EmailClaims:             []string{"email"},
		NameClaims:              []string{"name", "email"},
	}
	return openid.NewProvider(providerName, nil, config)
}
