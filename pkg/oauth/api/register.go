package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta1",
		&AccessToken{},
		&AccessTokenList{},
		&AuthorizeToken{},
		&AuthorizeTokenList{},
		&Client{},
		&ClientList{},
		&ClientAuthorization{},
		&ClientAuthorizationList{},
	)
}

func (*AccessToken) IsAnAPIObject()             {}
func (*AuthorizeToken) IsAnAPIObject()          {}
func (*Client) IsAnAPIObject()                  {}
func (*AccessTokenList) IsAnAPIObject()         {}
func (*AuthorizeTokenList) IsAnAPIObject()      {}
func (*ClientList) IsAnAPIObject()              {}
func (*ClientAuthorization) IsAnAPIObject()     {}
func (*ClientAuthorizationList) IsAnAPIObject() {}
