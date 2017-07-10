package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OAuthAccessToken describes an OAuth access token
type OAuthAccessToken struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// ClientName references the client that created this token.
	ClientName string `json:"clientName,omitempty" protobuf:"bytes,2,opt,name=clientName"`

	// ExpiresIn is the seconds from CreationTime before this token expires.
	ExpiresIn int64 `json:"expiresIn,omitempty" protobuf:"varint,3,opt,name=expiresIn"`

	// Scopes is an array of the requested scopes.
	Scopes []string `json:"scopes,omitempty" protobuf:"bytes,4,rep,name=scopes"`

	// RedirectURI is the redirection associated with the token.
	RedirectURI string `json:"redirectURI,omitempty" protobuf:"bytes,5,opt,name=redirectURI"`

	// UserName is the user name associated with this token
	UserName string `json:"userName,omitempty" protobuf:"bytes,6,opt,name=userName"`

	// UserUID is the unique UID associated with this token
	UserUID string `json:"userUID,omitempty" protobuf:"bytes,7,opt,name=userUID"`

	// AuthorizeToken contains the token that authorized this token
	AuthorizeToken string `json:"authorizeToken,omitempty" protobuf:"bytes,8,opt,name=authorizeToken"`

	// RefreshToken is the value by which this token can be renewed. Can be blank.
	RefreshToken string `json:"refreshToken,omitempty" protobuf:"bytes,9,opt,name=refreshToken"`
}

// OAuthAuthorizeToken describes an OAuth authorization token
type OAuthAuthorizeToken struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// ClientName references the client that created this token.
	ClientName string `json:"clientName,omitempty" protobuf:"bytes,2,opt,name=clientName"`

	// ExpiresIn is the seconds from CreationTime before this token expires.
	ExpiresIn int64 `json:"expiresIn,omitempty" protobuf:"varint,3,opt,name=expiresIn"`

	// Scopes is an array of the requested scopes.
	Scopes []string `json:"scopes,omitempty" protobuf:"bytes,4,rep,name=scopes"`

	// RedirectURI is the redirection associated with the token.
	RedirectURI string `json:"redirectURI,omitempty" protobuf:"bytes,5,opt,name=redirectURI"`

	// State data from request
	State string `json:"state,omitempty" protobuf:"bytes,6,opt,name=state"`

	// UserName is the user name associated with this token
	UserName string `json:"userName,omitempty" protobuf:"bytes,7,opt,name=userName"`

	// UserUID is the unique UID associated with this token. UserUID and UserName must both match
	// for this token to be valid.
	UserUID string `json:"userUID,omitempty" protobuf:"bytes,8,opt,name=userUID"`

	// CodeChallenge is the optional code_challenge associated with this authorization code, as described in rfc7636
	CodeChallenge string `json:"codeChallenge,omitempty" protobuf:"bytes,9,opt,name=codeChallenge"`

	// CodeChallengeMethod is the optional code_challenge_method associated with this authorization code, as described in rfc7636
	CodeChallengeMethod string `json:"codeChallengeMethod,omitempty" protobuf:"bytes,10,opt,name=codeChallengeMethod"`
}

// +genclient=true

// OAuthClient describes an OAuth client
type OAuthClient struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Secret is the unique secret associated with a client
	Secret string `json:"secret,omitempty" protobuf:"bytes,2,opt,name=secret"`

	// AdditionalSecrets holds other secrets that may be used to identify the client.  This is useful for rotation
	// and for service account token validation
	AdditionalSecrets []string `json:"additionalSecrets,omitempty" protobuf:"bytes,3,rep,name=additionalSecrets"`

	// RespondWithChallenges indicates whether the client wants authentication needed responses made in the form of challenges instead of redirects
	RespondWithChallenges bool `json:"respondWithChallenges,omitempty" protobuf:"varint,4,opt,name=respondWithChallenges"`

	// RedirectURIs is the valid redirection URIs associated with a client
	// +patchStrategy=merge
	RedirectURIs []string `json:"redirectURIs,omitempty" patchStrategy:"merge" protobuf:"bytes,5,rep,name=redirectURIs"`

	// GrantMethod determines how to handle grants for this client. If no method is provided, the
	// cluster default grant handling method will be used. Valid grant handling methods are:
	//  - auto:   always approves grant requests, useful for trusted clients
	//  - prompt: prompts the end user for approval of grant requests, useful for third-party clients
	//  - deny:   always denies grant requests, useful for black-listed clients
	GrantMethod GrantHandlerType `json:"grantMethod,omitempty" protobuf:"bytes,6,opt,name=grantMethod,casttype=GrantHandlerType"`

	// ScopeRestrictions describes which scopes this client can request.  Each requested scope
	// is checked against each restriction.  If any restriction matches, then the scope is allowed.
	// If no restriction matches, then the scope is denied.
	ScopeRestrictions []ScopeRestriction `json:"scopeRestrictions,omitempty" protobuf:"bytes,7,rep,name=scopeRestrictions"`
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
	ExactValues []string `json:"literals,omitempty" protobuf:"bytes,1,rep,name=literals"`

	// ClusterRole describes a set of restrictions for cluster role scoping.
	ClusterRole *ClusterRoleScopeRestriction `json:"clusterRole,omitempty" protobuf:"bytes,2,opt,name=clusterRole"`
}

// ClusterRoleScopeRestriction describes restrictions on cluster role scopes
type ClusterRoleScopeRestriction struct {
	// RoleNames is the list of cluster roles that can referenced.  * means anything
	RoleNames []string `json:"roleNames" protobuf:"bytes,1,rep,name=roleNames"`
	// Namespaces is the list of namespaces that can be referenced.  * means any of them (including *)
	Namespaces []string `json:"namespaces" protobuf:"bytes,2,rep,name=namespaces"`
	// AllowEscalation indicates whether you can request roles and their escalating resources
	AllowEscalation bool `json:"allowEscalation" protobuf:"varint,3,opt,name=allowEscalation"`
}

// OAuthClientAuthorization describes an authorization created by an OAuth client
type OAuthClientAuthorization struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// ClientName references the client that created this authorization
	ClientName string `json:"clientName,omitempty" protobuf:"bytes,2,opt,name=clientName"`

	// UserName is the user name that authorized this client
	UserName string `json:"userName,omitempty" protobuf:"bytes,3,opt,name=userName"`

	// UserUID is the unique UID associated with this authorization. UserUID and UserName
	// must both match for this authorization to be valid.
	UserUID string `json:"userUID,omitempty" protobuf:"bytes,4,opt,name=userUID"`

	// Scopes is an array of the granted scopes.
	Scopes []string `json:"scopes,omitempty" protobuf:"bytes,5,rep,name=scopes"`
}

// OAuthAccessTokenList is a collection of OAuth access tokens
type OAuthAccessTokenList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of OAuth access tokens
	Items []OAuthAccessToken `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// OAuthAuthorizeTokenList is a collection of OAuth authorization tokens
type OAuthAuthorizeTokenList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of OAuth authorization tokens
	Items []OAuthAuthorizeToken `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// OAuthClientList is a collection of OAuth clients
type OAuthClientList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of OAuth clients
	Items []OAuthClient `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// OAuthClientAuthorizationList is a collection of OAuth client authorizations
type OAuthClientAuthorizationList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of OAuth client authorizations
	Items []OAuthClientAuthorization `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// OAuthRedirectReference is a reference to an OAuth redirect object.
type OAuthRedirectReference struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// The reference to an redirect object in the current namespace.
	Reference RedirectReference `json:"reference,omitempty" protobuf:"bytes,2,opt,name=reference"`
}

// RedirectReference specifies the target in the current namespace that resolves into redirect URIs.  Only the 'Route' kind is currently allowed.
type RedirectReference struct {
	// The group of the target that is being referred to.
	Group string `json:"group" protobuf:"bytes,1,opt,name=group"`

	// The kind of the target that is being referred to.  Currently, only 'Route' is allowed.
	Kind string `json:"kind" protobuf:"bytes,2,opt,name=kind"`

	// The name of the target that is being referred to. e.g. name of the Route.
	Name string `json:"name" protobuf:"bytes,3,opt,name=name"`
}
