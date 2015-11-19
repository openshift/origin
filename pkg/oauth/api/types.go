package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

type OAuthAccessToken struct {
	unversioned.TypeMeta
	kapi.ObjectMeta

	// ClientName references the client that created this token.
	ClientName string

	// ExpiresIn is the seconds from CreationTime before this token expires.
	ExpiresIn int64

	// Scopes is an array of the requested scopes.
	Scopes []string

	// RedirectURI is the redirection associated with the token.
	RedirectURI string

	// UserName is the user name associated with this token
	UserName string

	// UserUID is the unique UID associated with this token
	UserUID string

	// AuthorizeToken contains the token that authorized this token
	AuthorizeToken string

	// RefreshToken is the value by which this token can be renewed. Can be blank.
	RefreshToken string
}

type OAuthAuthorizeToken struct {
	unversioned.TypeMeta
	kapi.ObjectMeta

	// ClientName references the client that created this token.
	ClientName string

	// ExpiresIn is the seconds from CreationTime before this token expires.
	ExpiresIn int64

	// Scopes is an array of the requested scopes.
	Scopes []string

	// RedirectURI is the redirection associated with the token.
	RedirectURI string

	// State data from request
	State string

	// UserName is the user name associated with this token
	UserName string

	// UserUID is the unique UID associated with this token. UserUID and UserName must both match
	// for this token to be valid.
	UserUID string
}

type OAuthClient struct {
	unversioned.TypeMeta
	kapi.ObjectMeta

	// Secret is the unique secret associated with a client
	Secret string

	// RespondWithChallenges indicates whether the client wants authentication needed responses made in the form of challenges instead of redirects
	RespondWithChallenges bool

	// RedirectURIs is the valid redirection URIs associated with a client
	RedirectURIs []string
}

type OAuthClientAuthorization struct {
	unversioned.TypeMeta
	kapi.ObjectMeta

	// ClientName references the client that created this authorization
	ClientName string

	// UserName is the user name that authorized this client
	UserName string

	// UserUID is the unique UID associated with this authorization. UserUID and UserName
	// must both match for this authorization to be valid.
	UserUID string

	// Scopes is an array of the granted scopes.
	Scopes []string
}

type OAuthAccessTokenList struct {
	unversioned.TypeMeta
	unversioned.ListMeta
	Items []OAuthAccessToken
}

type OAuthAuthorizeTokenList struct {
	unversioned.TypeMeta
	unversioned.ListMeta
	Items []OAuthAuthorizeToken
}

type OAuthClientList struct {
	unversioned.TypeMeta
	unversioned.ListMeta
	Items []OAuthClient
}

type OAuthClientAuthorizationList struct {
	unversioned.TypeMeta
	unversioned.ListMeta
	Items []OAuthClientAuthorization
}
