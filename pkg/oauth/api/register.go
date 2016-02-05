package api

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: ""}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) unversioned.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns back a Group qualified GroupResource
func Resource(resource string) unversioned.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

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
