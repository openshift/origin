package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

type AccessToken struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`

	// AuthorizeToken is the authorization token that granted this access token, and contains
	// the specific state of the token.
	AuthorizeToken AuthorizeToken `json:"authorizeToken,omitempty" yaml:"authorizeToken,omitempty"`

	// RefreshToken is the value by which this token can be renewed. Can be blank.
	RefreshToken string `json:"refreshToken,omitempty" yaml:"refreshToken,omitempty"`
}

type AuthorizeToken struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// ClientName references the client that created this token.
	ClientName string `json:"clientName,omitempty" yaml:"clientName,omitempty"`

	// ExpiresIn is the seconds from CreationTime before this token expires.
	ExpiresIn int64 `json:"expiresIn,omitempty" yaml:"expiresIn,omitempty"`

	// Scopes is an array of the requested scopes.
	Scopes []string `json:"scopes,omitempty" yaml:"scopes,omitempty"`

	// RedirectURI is the redirection associated with the token.
	RedirectURI string `json:"redirectURI,omitempty" yaml:"redirectURI,omitempty"`

	// State data from request
	State string `json:"state,omitempty" yaml:"state,omitempty"`

	// UserName is the user name associated with this token
	UserName string `json:"userName,omitempty" yaml:"userName,omitempty"`

	// UserUID is the unique UID associated with this token. UserUID and UserName must both match
	// for this token to be valid.
	UserUID string `json:"userUID,omitempty" yaml:"userUID,omitempty"`
}

type Client struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`

	// Secret is the unique secret associated with a client
	Secret string `json:"secret,omitempty" yaml:"secret,omitempty"`

	// RedirectURIs is the valid redirection URIs associated with a client
	RedirectURIs []string `json:"redirectURIs,omitempty" yaml:"redirectURIs,omitempty"`
}

type ClientAuthorization struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// ClientName references the client that created this authorization
	ClientName string `json:"clientName,omitempty" yaml:"clientName,omitempty"`

	// UserName is the user name that authorized this client
	UserName string `json:"userName,omitempty" yaml:"userName,omitempty"`

	// UserUID is the unique UID associated with this authorization. UserUID and UserName
	// must both match for this authorization to be valid.
	UserUID string `json:"userUID,omitempty" yaml:"userUID,omitempty"`

	// Scopes is an array of the granted scopes.
	Scopes []string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
}

type AccessTokenList struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []AccessToken `json:"items,omitempty" yaml:"items,omitempty"`
}

type AuthorizeTokenList struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []AuthorizeToken `json:"items,omitempty" yaml:"items,omitempty"`
}

type ClientList struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []Client `json:"items,omitempty" yaml:"items,omitempty"`
}

type ClientAuthorizationList struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []ClientAuthorization `json:"items,omitempty" yaml:"items,omitempty"`
}

func (*AccessToken) IsAnAPIObject()             {}
func (*AuthorizeToken) IsAnAPIObject()          {}
func (*Client) IsAnAPIObject()                  {}
func (*AccessTokenList) IsAnAPIObject()         {}
func (*AuthorizeTokenList) IsAnAPIObject()      {}
func (*ClientList) IsAnAPIObject()              {}
func (*ClientAuthorization) IsAnAPIObject()     {}
func (*ClientAuthorizationList) IsAnAPIObject() {}
