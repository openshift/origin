package v1

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: "v1"}

// Codec encodes internal objects to the v1 scheme
var Codec = runtime.CodecFor(api.Scheme, SchemeGroupVersion.String())

func init() {
	api.Scheme.AddKnownTypes(SchemeGroupVersion,
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
