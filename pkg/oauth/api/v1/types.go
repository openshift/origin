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

// +genclient=true

// OAuthClient describes an OAuth client
type OAuthClient struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Secret is the unique secret associated with a client
	Secret string `json:"secret,omitempty"`

	// AdditionalSecrets holds other secrets that may be used to identify the client.  This is useful for rotation
	// and for service account token validation
	AdditionalSecrets []string `json:"additionalSecrets,omitempty"`

	// RespondWithChallenges indicates whether the client wants authentication needed responses made in the form of challenges instead of redirects
	RespondWithChallenges bool `json:"respondWithChallenges,omitempty"`

	// RedirectURIs is the valid redirection URIs associated with a client
	RedirectURIs []string `json:"redirectURIs,omitempty"`

	// ScopeRestrictions describes which scopes this client can request.  Each requested scope
	// is checked against each restriction.  If any restriction matches, then the scope is allowed.
	// If no restriction matches, then the scope is denied.
	ScopeRestrictions []ScopeRestriction `json:"scopeRestrictions,omitempty"`
}

// ScopeRestriction describe one restriction on scopes.  Exactly one option must be non-nil.
type ScopeRestriction struct {
	// ExactValues means the scope has to match a particular set of strings exactly
	ExactValues []string `json:"literals,omitempty"`

	// ClusterRole describes a set of restrictions for cluster role scoping.
	ClusterRole *ClusterRoleScopeRestriction `json:"clusterRole,omitempty"`
}

// ClusterRoleScopeRestriction describes restrictions on cluster role scopes
type ClusterRoleScopeRestriction struct {
	// RoleNames is the list of cluster roles that can referenced.  * means anything
	RoleNames []string `json:"roleNames"`
	// Namespaces is the list of namespaces that can be referenced.  * means any of them (including *)
	Namespaces []string `json:"namespaces"`
	// AllowEscalation indicates whether you can request roles and their escalating resources
	AllowEscalation bool `json:"allowEscalation"`
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
