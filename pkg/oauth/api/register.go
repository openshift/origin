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
