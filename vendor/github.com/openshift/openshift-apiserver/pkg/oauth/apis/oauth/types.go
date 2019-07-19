package oauth

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OAuthAccessToken struct {
	metav1.TypeMeta
	metav1.ObjectMeta

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

	// InactivityTimeoutSeconds is the value in seconds, from the
	// CreationTimestamp, after which this token can no longer be used.
	// The value is automatically incremented when the token is used.
	InactivityTimeoutSeconds int32
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OAuthAuthorizeToken struct {
	metav1.TypeMeta
	metav1.ObjectMeta

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

	// CodeChallenge is the optional code_challenge associated with this authorization code, as described in rfc7636
	CodeChallenge string

	// CodeChallengeMethod is the optional code_challenge_method associated with this authorization code, as described in rfc7636
	CodeChallengeMethod string
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OAuthClient struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Secret is the unique secret associated with a client
	Secret string

	// AdditionalSecrets holds other secrets that may be used to identify the client.  This is useful for rotation
	// and for service account token validation
	AdditionalSecrets []string

	// RespondWithChallenges indicates whether the client wants authentication needed responses made in the form of challenges instead of redirects
	RespondWithChallenges bool

	// RedirectURIs is the valid redirection URIs associated with a client
	RedirectURIs []string

	// GrantMethod determines how to handle grants for this client. If no method is provided, the
	// cluster default grant handling method will be used
	GrantMethod GrantHandlerType

	// ScopeRestrictions describes which scopes this client can request.  Each requested scope
	// is checked against each restriction.  If any restriction matches, then the scope is allowed.
	// If no restriction matches, then the scope is denied.
	ScopeRestrictions []ScopeRestriction

	// AccessTokenMaxAgeSeconds overrides the default access token max age for tokens granted to this client.
	// 0 means no expiration.
	AccessTokenMaxAgeSeconds *int32

	// AccessTokenInactivityTimeoutSeconds overrides the default token
	// inactivity timeout for tokens granted to this client.
	// The value represents the maximum amount of time that can occur between
	// consecutive uses of the token. Tokens become invalid if they are not
	// used within this temporal window. The user will need to acquire a new
	// token to regain access once a token times out.
	// This value needs to be set only if the default set in configuration is
	// not appropriate for this client. Valid values are:
	// - 0: Tokens for this client never time out
	// - X: Tokens time out if there is no activity for X seconds
	// The current minimum allowed value for X is 300 (5 minutes)
	AccessTokenInactivityTimeoutSeconds *int32
}

type GrantHandlerType string

const (
	// GrantHandlerAuto auto-approves client authorization grant requests
	GrantHandlerAuto GrantHandlerType = "auto"
	// GrantHandlerPrompt prompts the user to approve new client authorization grant requests
	GrantHandlerPrompt GrantHandlerType = "prompt"
	// GrantHandlerDeny auto-denies client authorization grant requests
	GrantHandlerDeny GrantHandlerType = "deny"
)

// ScopeRestriction describe one restriction on scopes.  Exactly one option must be non-nil.
type ScopeRestriction struct {
	// ExactValues means the scope has to match a particular set of strings exactly
	ExactValues []string

	// ClusterRole describes a set of restrictions for cluster role scoping.
	ClusterRole *ClusterRoleScopeRestriction
}

// ClusterRoleScopeRestriction describes restrictions on cluster role scopes
type ClusterRoleScopeRestriction struct {
	// RoleNames is the list of cluster roles that can referenced.  * means anything
	RoleNames []string
	// Namespaces is the list of namespaces that can be referenced.  * means any of them (including *)
	Namespaces []string
	// AllowEscalation indicates whether you can request roles and their escalating resources
	AllowEscalation bool
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OAuthClientAuthorization struct {
	metav1.TypeMeta
	metav1.ObjectMeta

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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OAuthAccessTokenList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []OAuthAccessToken
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OAuthAuthorizeTokenList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []OAuthAuthorizeToken
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OAuthClientList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []OAuthClient
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OAuthClientAuthorizationList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []OAuthClientAuthorization
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OAuthRedirectReference struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	Reference RedirectReference
}

type RedirectReference struct {
	Group string
	Kind  string
	Name  string
}
