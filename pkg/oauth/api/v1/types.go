package v1

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1"
)

type OAuthAccessToken struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// ClientName references the client that created this token.
	ClientName string `json:"clientName,omitempty" description:"references the client that created this token"`

	// ExpiresIn is the seconds from CreationTime before this token expires.
	ExpiresIn int64 `json:"expiresIn,omitempty" description:"is the seconds from creation time before this token expires"`

	// Scopes is an array of the requested scopes.
	Scopes []string `json:"scopes,omitempty" description:"list of requested scopes"`

	// RedirectURI is the redirection associated with the token.
	RedirectURI string `json:"redirectURI,omitempty" description:"redirection URI associated with the token"`

	// UserName is the user name associated with this token
	UserName string `json:"userName,omitempty" description:"user name associated with this token"`

	// UserUID is the unique UID associated with this token
	UserUID string `json:"userUID,omitempty" description:"unique UID associated with this token"`

	// AuthorizeToken contains the token that authorized this token
	AuthorizeToken string `json:"authorizeToken,omitempty" description:"contains the token that authorized this token"`

	// RefreshToken is the value by which this token can be renewed. Can be blank.
	RefreshToken string `json:"refreshToken,omitempty" description:"optional value by which this token can be renewed"`
}

type OAuthAuthorizeToken struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// ClientName references the client that created this token.
	ClientName string `json:"clientName,omitempty" description:"references the client that created this token"`

	// ExpiresIn is the seconds from CreationTime before this token expires.
	ExpiresIn int64 `json:"expiresIn,omitempty" description:"seconds from creation time before this token expires"`

	// Scopes is an array of the requested scopes.
	Scopes []string `json:"scopes,omitempty" description:"list of requested scopes"`

	// RedirectURI is the redirection associated with the token.
	RedirectURI string `json:"redirectURI,omitempty" description:"redirection URI associated with the token"`

	// State data from request
	State string `json:"state,omitempty" description:"state data from request"`

	// UserName is the user name associated with this token
	UserName string `json:"userName,omitempty" description:"user name associated with this token"`

	// UserUID is the unique UID associated with this token. UserUID and UserName must both match
	// for this token to be valid.
	UserUID string `json:"userUID,omitempty" description:"unique UID associated with this token.  userUID and userName must both match for this token to be valid"`
}

type OAuthClient struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Secret is the unique secret associated with a client
	Secret string `json:"secret,omitempty" description:"unique secret associated with a client"`

	// RespondWithChallenges indicates whether the client wants authentication needed responses made in the form of challenges instead of redirects
	RespondWithChallenges bool `json:"respondWithChallenges,omitempty" description:"indicates whether the client wants authentication needed responses made in the form of challenges instead of redirects"`

	// RedirectURIs is the valid redirection URIs associated with a client
	RedirectURIs []string `json:"redirectURIs,omitempty" description:"valid redirection URIs associated with a client"`
}

type OAuthClientAuthorization struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// ClientName references the client that created this authorization
	ClientName string `json:"clientName,omitempty" description:"references the client that created this authorization"`

	// UserName is the user name that authorized this client
	UserName string `json:"userName,omitempty" description:"user name that authorized this client"`

	// UserUID is the unique UID associated with this authorization. UserUID and UserName
	// must both match for this authorization to be valid.
	UserUID string `json:"userUID,omitempty" description:"unique UID associated with this authorization. userUID and userName must both match for this authorization to be valid"`

	// Scopes is an array of the granted scopes.
	Scopes []string `json:"scopes,omitempty" description:"list of granted scopes"`
}

type OAuthAccessTokenList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []OAuthAccessToken `json:"items" description:"list of oauth access tokens"`
}

type OAuthAuthorizeTokenList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []OAuthAuthorizeToken `json:"items" description:"list of oauth authorization tokens"`
}

type OAuthClientList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []OAuthClient `json:"items" description:"list of oauth clients"`
}

type OAuthClientAuthorizationList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []OAuthClientAuthorization `json:"items" description:"list of oauth client authorizations"`
}
