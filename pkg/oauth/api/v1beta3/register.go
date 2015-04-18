package v1beta2

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta2",
		&OAuthAccessToken{},
		&OAuthAccessTokenList{},
		&OAuthAuthorizeToken{},
		&OAuthAuthorizeTokenList{},
		&OAuthClient{},
		&OAuthClientList{},
		&OAuthClientAuthorization{},
		&OAuthClientAuthorizationList{},
	)
}

func (*OAuthAccessToken) IsAnAPIObject()             {}
func (*OAuthAuthorizeToken) IsAnAPIObject()          {}
func (*OAuthClient) IsAnAPIObject()                  {}
func (*OAuthAccessTokenList) IsAnAPIObject()         {}
func (*OAuthAuthorizeTokenList) IsAnAPIObject()      {}
func (*OAuthClientList) IsAnAPIObject()              {}
func (*OAuthClientAuthorization) IsAnAPIObject()     {}
func (*OAuthClientAuthorizationList) IsAnAPIObject() {}
