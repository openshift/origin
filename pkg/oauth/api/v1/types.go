package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1"
)

// OAuthAccessToken describes an OAuth access token
type OAuthAccessToken struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// ClientName references the client that created this token.
	ClientName string `json:"clientName,omitempty"`

	// ExpiresIn is the seconds from CreationTime before this token expires.
	ExpiresIn int64 `json:"expiresIn,omitempty"`

	// Scopes is an array of the requested scopes.
	Scopes []string `json:"scopes,omitempty"`

	// RedirectURI is the redirection associated with the token.
	RedirectURI string `json:"redirectURI,omitempty"`

	// UserName is the user name associated with this token
	UserName string `json:"userName,omitempty"`

	// UserUID is the unique UID associated with this token
	UserUID string `json:"userUID,omitempty"`

	// AuthorizeToken contains the token that authorized this token
	AuthorizeToken string `json:"authorizeToken,omitempty"`

	// RefreshToken is the value by which this token can be renewed. Can be blank.
	RefreshToken string `json:"refreshToken,omitempty"`
}

// OAuthAuthorizeToken describes an OAuth authorization token
type OAuthAuthorizeToken struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// ClientName references the client that created this token.
	ClientName string `json:"clientName,omitempty"`

	// ExpiresIn is the seconds from CreationTime before this token expires.
	ExpiresIn int64 `json:"expiresIn,omitempty"`

	// Scopes is an array of the requested scopes.
	Scopes []string `json:"scopes,omitempty"`

	// RedirectURI is the redirection associated with the token.
	RedirectURI string `json:"redirectURI,omitempty"`

	// State data from request
	State string `json:"state,omitempty"`

	// UserName is the user name associated with this token
	UserName string `json:"userName,omitempty"`

	// UserUID is the unique UID associated with this token. UserUID and UserName must both match
	// for this token to be valid.
	UserUID string `json:"userUID,omitempty"`
}

// OAuthClient describes an OAuth client
type OAuthClient struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Secret is the unique secret associated with a client
	Secret string `json:"secret,omitempty"`

	// RespondWithChallenges indicates whether the client wants authentication needed responses made in the form of challenges instead of redirects
	RespondWithChallenges bool `json:"respondWithChallenges,omitempty"`

	// RedirectURIs is the valid redirection URIs associated with a client
	RedirectURIs []string `json:"redirectURIs,omitempty"`
}

// OAuthClientAuthorization describes an authorization created by an OAuth client
type OAuthClientAuthorization struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// ClientName references the client that created this authorization
	ClientName string `json:"clientName,omitempty"`

	// UserName is the user name that authorized this client
	UserName string `json:"userName,omitempty"`

	// UserUID is the unique UID associated with this authorization. UserUID and UserName
	// must both match for this authorization to be valid.
	UserUID string `json:"userUID,omitempty"`

	// Scopes is an array of the granted scopes.
	Scopes []string `json:"scopes,omitempty"`
}

// OAuthAccessTokenList is a collection of OAuth access tokens
type OAuthAccessTokenList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty"`
	// Items is the list of OAuth access tokens
	Items []OAuthAccessToken `json:"items"`
}

// OAuthAuthorizeTokenList is a collection of OAuth authorization tokens
type OAuthAuthorizeTokenList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty"`
	// Items is the list of OAuth authorization tokens
	Items []OAuthAuthorizeToken `json:"items"`
}

// OAuthClientList is a collection of OAuth clients
type OAuthClientList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty"`
	// Items is the list of OAuth clients
	Items []OAuthClient `json:"items"`
}

// OAuthClientAuthorizationList is a collection of OAuth client authorizations
type OAuthClientAuthorizationList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty"`
	// Items is the list of OAuth client authorizations
	Items []OAuthClientAuthorization `json:"items"`
}
