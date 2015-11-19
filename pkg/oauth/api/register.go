package api

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
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
