package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +openshift:validation:FeatureGateAwareXValidation:featureGate=ExternalOIDC;ExternalOIDCWithUIDAndExtraClaimMappings,rule="!has(self.spec.oidcProviders) || self.spec.oidcProviders.all(p, !has(p.oidcClients) || p.oidcClients.all(specC, self.status.oidcClients.exists(statusC, statusC.componentNamespace == specC.componentNamespace && statusC.componentName == specC.componentName) || (has(oldSelf.spec.oidcProviders) && oldSelf.spec.oidcProviders.exists(oldP, oldP.name == p.name && has(oldP.oidcClients) && oldP.oidcClients.exists(oldC, oldC.componentNamespace == specC.componentNamespace && oldC.componentName == specC.componentName)))))",message="all oidcClients in the oidcProviders must match their componentName and componentNamespace to either a previously configured oidcClient or they must exist in the status.oidcClients"

// Authentication specifies cluster-wide settings for authentication (like OAuth and
// webhook token authenticators). The canonical name of an instance is `cluster`.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/470
// +openshift:file-pattern=cvoRunLevel=0000_10,operatorName=config-operator,operatorOrdering=01
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=authentications,scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:metadata:annotations=release.openshift.io/bootstrap-required=true
type Authentication struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	// +required
	Spec AuthenticationSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	// +optional
	Status AuthenticationStatus `json:"status"`
}

type AuthenticationSpec struct {
	// type identifies the cluster managed, user facing authentication mode in use.
	// Specifically, it manages the component that responds to login attempts.
	// The default is IntegratedOAuth.
	// +optional
	Type AuthenticationType `json:"type"`

	// oauthMetadata contains the discovery endpoint data for OAuth 2.0
	// Authorization Server Metadata for an external OAuth server.
	// This discovery document can be viewed from its served location:
	// oc get --raw '/.well-known/oauth-authorization-server'
	// For further details, see the IETF Draft:
	// https://tools.ietf.org/html/draft-ietf-oauth-discovery-04#section-2
	// If oauthMetadata.name is non-empty, this value has precedence
	// over any metadata reference stored in status.
	// The key "oauthMetadata" is used to locate the data.
	// If specified and the config map or expected key is not found, no metadata is served.
	// If the specified metadata is not valid, no metadata is served.
	// The namespace for this config map is openshift-config.
	// +optional
	OAuthMetadata ConfigMapNameReference `json:"oauthMetadata"`

	// webhookTokenAuthenticators is DEPRECATED, setting it has no effect.
	// +listType=atomic
	WebhookTokenAuthenticators []DeprecatedWebhookTokenAuthenticator `json:"webhookTokenAuthenticators,omitempty"`

	// webhookTokenAuthenticator configures a remote token reviewer.
	// These remote authentication webhooks can be used to verify bearer tokens
	// via the tokenreviews.authentication.k8s.io REST API. This is required to
	// honor bearer tokens that are provisioned by an external authentication service.
	//
	// Can only be set if "Type" is set to "None".
	//
	// +optional
	WebhookTokenAuthenticator *WebhookTokenAuthenticator `json:"webhookTokenAuthenticator,omitempty"`

	// serviceAccountIssuer is the identifier of the bound service account token
	// issuer.
	// The default is https://kubernetes.default.svc
	// WARNING: Updating this field will not result in immediate invalidation of all bound tokens with the
	// previous issuer value. Instead, the tokens issued by previous service account issuer will continue to
	// be trusted for a time period chosen by the platform (currently set to 24h).
	// This time period is subject to change over time.
	// This allows internal components to transition to use new service account issuer without service distruption.
	// +optional
	ServiceAccountIssuer string `json:"serviceAccountIssuer"`

	// oidcProviders are OIDC identity providers that can issue tokens
	// for this cluster
	// Can only be set if "Type" is set to "OIDC".
	//
	// At most one provider can be configured.
	//
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=1
	// +openshift:enable:FeatureGate=ExternalOIDC
	// +openshift:enable:FeatureGate=ExternalOIDCWithUIDAndExtraClaimMappings
	OIDCProviders []OIDCProvider `json:"oidcProviders,omitempty"`
}

type AuthenticationStatus struct {
	// integratedOAuthMetadata contains the discovery endpoint data for OAuth 2.0
	// Authorization Server Metadata for the in-cluster integrated OAuth server.
	// This discovery document can be viewed from its served location:
	// oc get --raw '/.well-known/oauth-authorization-server'
	// For further details, see the IETF Draft:
	// https://tools.ietf.org/html/draft-ietf-oauth-discovery-04#section-2
	// This contains the observed value based on cluster state.
	// An explicitly set value in spec.oauthMetadata has precedence over this field.
	// This field has no meaning if authentication spec.type is not set to IntegratedOAuth.
	// The key "oauthMetadata" is used to locate the data.
	// If the config map or expected key is not found, no metadata is served.
	// If the specified metadata is not valid, no metadata is served.
	// The namespace for this config map is openshift-config-managed.
	IntegratedOAuthMetadata ConfigMapNameReference `json:"integratedOAuthMetadata"`

	// oidcClients is where participating operators place the current OIDC client status
	// for OIDC clients that can be customized by the cluster-admin.
	//
	// +listType=map
	// +listMapKey=componentNamespace
	// +listMapKey=componentName
	// +kubebuilder:validation:MaxItems=20
	// +openshift:enable:FeatureGate=ExternalOIDC
	// +openshift:enable:FeatureGate=ExternalOIDCWithUIDAndExtraClaimMappings
	OIDCClients []OIDCClientStatus `json:"oidcClients"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type AuthenticationList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []Authentication `json:"items"`
}

// +openshift:validation:FeatureGateAwareEnum:featureGate="",enum="";None;IntegratedOAuth
// +openshift:validation:FeatureGateAwareEnum:featureGate=ExternalOIDC;ExternalOIDCWithUIDAndExtraClaimMappings,enum="";None;IntegratedOAuth;OIDC
type AuthenticationType string

const (
	// None means that no cluster managed authentication system is in place.
	// Note that user login will only work if a manually configured system is in place and
	// referenced in authentication spec via oauthMetadata and
	// webhookTokenAuthenticator/oidcProviders
	AuthenticationTypeNone AuthenticationType = "None"

	// IntegratedOAuth refers to the cluster managed OAuth server.
	// It is configured via the top level OAuth config.
	AuthenticationTypeIntegratedOAuth AuthenticationType = "IntegratedOAuth"

	// AuthenticationTypeOIDC refers to a configuration with an external
	// OIDC server configured directly with the kube-apiserver.
	AuthenticationTypeOIDC AuthenticationType = "OIDC"
)

// deprecatedWebhookTokenAuthenticator holds the necessary configuration options for a remote token authenticator.
// It's the same as WebhookTokenAuthenticator but it's missing the 'required' validation on KubeConfig field.
type DeprecatedWebhookTokenAuthenticator struct {
	// kubeConfig contains kube config file data which describes how to access the remote webhook service.
	// For further details, see:
	// https://kubernetes.io/docs/reference/access-authn-authz/authentication/#webhook-token-authentication
	// The key "kubeConfig" is used to locate the data.
	// If the secret or expected key is not found, the webhook is not honored.
	// If the specified kube config data is not valid, the webhook is not honored.
	// The namespace for this secret is determined by the point of use.
	KubeConfig SecretNameReference `json:"kubeConfig"`
}

// webhookTokenAuthenticator holds the necessary configuration options for a remote token authenticator
type WebhookTokenAuthenticator struct {
	// kubeConfig references a secret that contains kube config file data which
	// describes how to access the remote webhook service.
	// The namespace for the referenced secret is openshift-config.
	//
	// For further details, see:
	//
	// https://kubernetes.io/docs/reference/access-authn-authz/authentication/#webhook-token-authentication
	//
	// The key "kubeConfig" is used to locate the data.
	// If the secret or expected key is not found, the webhook is not honored.
	// If the specified kube config data is not valid, the webhook is not honored.
	// +required
	KubeConfig SecretNameReference `json:"kubeConfig"`
}

const (
	// OAuthMetadataKey is the key for the oauth authorization server metadata
	OAuthMetadataKey = "oauthMetadata"

	// KubeConfigKey is the key for the kube config file data in a secret
	KubeConfigKey = "kubeConfig"
)

type OIDCProvider struct {
	// name of the OIDC provider
	//
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`
	// issuer describes atributes of the OIDC token issuer
	//
	// +required
	Issuer TokenIssuer `json:"issuer"`

	// oidcClients contains configuration for the platform's clients that
	// need to request tokens from the issuer
	//
	// +listType=map
	// +listMapKey=componentNamespace
	// +listMapKey=componentName
	// +kubebuilder:validation:MaxItems=20
	OIDCClients []OIDCClientConfig `json:"oidcClients"`

	// claimMappings describes rules on how to transform information from an
	// ID token into a cluster identity
	ClaimMappings TokenClaimMappings `json:"claimMappings"`

	// claimValidationRules are rules that are applied to validate token claims to authenticate users.
	//
	// +listType=atomic
	ClaimValidationRules []TokenClaimValidationRule `json:"claimValidationRules,omitempty"`
}

// +kubebuilder:validation:MinLength=1
type TokenAudience string

type TokenIssuer struct {
	// URL is the serving URL of the token issuer.
	// Must use the https:// scheme.
	//
	// +kubebuilder:validation:Pattern=`^https:\/\/[^\s]`
	// +required
	URL string `json:"issuerURL"`

	// audiences is an array of audiences that the token was issued for.
	// Valid tokens must include at least one of these values in their
	// "aud" claim.
	// Must be set to exactly one value.
	//
	// +listType=set
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=10
	// +required
	Audiences []TokenAudience `json:"audiences"`

	// CertificateAuthority is a reference to a config map in the
	// configuration namespace. The .data of the configMap must contain
	// the "ca-bundle.crt" key.
	// If unset, system trust is used instead.
	CertificateAuthority ConfigMapNameReference `json:"issuerCertificateAuthority"`
}

type TokenClaimMappings struct {
	// username is a name of the claim that should be used to construct
	// usernames for the cluster identity.
	//
	// Default value: "sub"
	Username UsernameClaimMapping `json:"username,omitempty"`

	// groups is a name of the claim that should be used to construct
	// groups for the cluster identity.
	// The referenced claim must use array of strings values.
	Groups PrefixedClaimMapping `json:"groups,omitempty"`

	// uid is an optional field for configuring the claim mapping
	// used to construct the uid for the cluster identity.
	//
	// When using uid.claim to specify the claim it must be a single string value.
	// When using uid.expression the expression must result in a single string value.
	//
	// When omitted, this means the user has no opinion and the platform
	// is left to choose a default, which is subject to change over time.
	// The current default is to use the 'sub' claim.
	//
	// +optional
	// +openshift:enable:FeatureGate=ExternalOIDCWithUIDAndExtraClaimMappings
	UID *TokenClaimOrExpressionMapping `json:"uid,omitempty"`

	// extra is an optional field for configuring the mappings
	// used to construct the extra attribute for the cluster identity.
	// When omitted, no extra attributes will be present on the cluster identity.
	// key values for extra mappings must be unique.
	// A maximum of 64 extra attribute mappings may be provided.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=64
	// +listType=map
	// +listMapKey=key
	// +openshift:enable:FeatureGate=ExternalOIDCWithUIDAndExtraClaimMappings
	Extra []ExtraMapping `json:"extra,omitempty"`
}

type TokenClaimMapping struct {
	// claim is a JWT token claim to be used in the mapping
	//
	// +required
	Claim string `json:"claim"`
}

// TokenClaimOrExpressionMapping allows specifying either a JWT
// token claim or CEL expression to be used when mapping claims
// from an authentication token to cluster identities.
// +kubebuilder:validation:XValidation:rule="has(self.claim) ? !has(self.expression) : has(self.expression)",message="precisely one of claim or expression must be set"
type TokenClaimOrExpressionMapping struct {
	// claim is an optional field for specifying the
	// JWT token claim that is used in the mapping.
	// The value of this claim will be assigned to
	// the field in which this mapping is associated.
	//
	// Precisely one of claim or expression must be set.
	// claim must not be specified when expression is set.
	// When specified, claim must be at least 1 character in length
	// and must not exceed 256 characters in length.
	//
	// +optional
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:MinLength=1
	Claim string `json:"claim,omitempty"`

	// expression is an optional field for specifying a
	// CEL expression that produces a string value from
	// JWT token claims.
	//
	// CEL expressions have access to the token claims
	// through a CEL variable, 'claims'.
	// 'claims' is a map of claim names to claim values.
	// For example, the 'sub' claim value can be accessed as 'claims.sub'.
	// Nested claims can be accessed using dot notation ('claims.foo.bar').
	//
	// Precisely one of claim or expression must be set.
	// expression must not be specified when claim is set.
	// When specified, expression must be at least 1 character in length
	// and must not exceed 4096 characters in length.
	//
	// +optional
	// +kubebuilder:validation:MaxLength=4096
	// +kubebuilder:validation:MinLength=1
	Expression string `json:"expression,omitempty"`
}

// ExtraMapping allows specifying a key and CEL expression
// to evaluate the keys' value. It is used to create additional
// mappings and attributes added to a cluster identity from
// a provided authentication token.
type ExtraMapping struct {
	// key is a required field that specifies the string
	// to use as the extra attribute key.
	//
	// key must be a domain-prefix path (e.g 'example.org/foo').
	// key must not exceed 510 characters in length.
	// key must contain the '/' character, separating the domain and path characters.
	// key must not be empty.
	//
	// The domain portion of the key (string of characters prior to the '/') must be a valid RFC1123 subdomain.
	// It must not exceed 253 characters in length.
	// It must start and end with an alphanumeric character.
	// It must only contain lower case alphanumeric characters and '-' or '.'.
	// It must not use the reserved domains, or be subdomains of, "kubernetes.io", "k8s.io", and "openshift.io".
	//
	// The path portion of the key (string of characters after the '/') must not be empty and must consist of at least one
	// alphanumeric character, percent-encoded octets, '-', '.', '_', '~', '!', '$', '&', ''', '(', ')', '*', '+', ',', ';', '=', and ':'.
	// It must not exceed 256 characters in length.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=510
	// +kubebuilder:validation:XValidation:rule="self.contains('/')",message="key must contain the '/' character"
	//
	// +kubebuilder:validation:XValidation:rule="self.split('/', 2)[0].matches(\"^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$\")",message="the domain of the key must consist of only lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character"
	// +kubebuilder:validation:XValidation:rule="self.split('/', 2)[0].size() <= 253",message="the domain of the key must not exceed 253 characters in length"
	//
	// +kubebuilder:validation:XValidation:rule="self.split('/', 2)[0] != 'kubernetes.io'",message="the domain 'kubernetes.io' is reserved for Kubernetes use"
	// +kubebuilder:validation:XValidation:rule="!self.split('/', 2)[0].endsWith('.kubernetes.io')",message="the subdomains '*.kubernetes.io' are reserved for Kubernetes use"
	// +kubebuilder:validation:XValidation:rule="self.split('/', 2)[0] != 'k8s.io'",message="the domain 'k8s.io' is reserved for Kubernetes use"
	// +kubebuilder:validation:XValidation:rule="!self.split('/', 2)[0].endsWith('.k8s.io')",message="the subdomains '*.k8s.io' are reserved for Kubernetes use"
	// +kubebuilder:validation:XValidation:rule="self.split('/', 2)[0] != 'openshift.io'",message="the domain 'openshift.io' is reserved for OpenShift use"
	// +kubebuilder:validation:XValidation:rule="!self.split('/', 2)[0].endsWith('.openshift.io')",message="the subdomains '*.openshift.io' are reserved for OpenShift use"
	//
	// +kubebuilder:validation:XValidation:rule="self.split('/', 2)[1].matches('[A-Za-z0-9/\\\\-._~%!$&\\'()*+;=:]+')",message="the path of the key must not be empty and must consist of at least one alphanumeric character, percent-encoded octets, apostrophe, '-', '.', '_', '~', '!', '$', '&', '(', ')', '*', '+', ',', ';', '=', and ':'"
	// +kubebuilder:validation:XValidation:rule="self.split('/', 2)[1].size() <= 256",message="the path of the key must not exceed 256 characters in length"
	Key string `json:"key"`

	// valueExpression is a required field to specify the CEL expression to extract
	// the extra attribute value from a JWT token's claims.
	// valueExpression must produce a string or string array value.
	// "", [], and null are treated as the extra mapping not being present.
	// Empty string values within an array are filtered out.
	//
	// CEL expressions have access to the token claims
	// through a CEL variable, 'claims'.
	// 'claims' is a map of claim names to claim values.
	// For example, the 'sub' claim value can be accessed as 'claims.sub'.
	// Nested claims can be accessed using dot notation ('claims.foo.bar').
	//
	// valueExpression must not exceed 4096 characters in length.
	// valueExpression must not be empty.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	ValueExpression string `json:"valueExpression"`
}

type OIDCClientConfig struct {
	// componentName is the name of the component that is supposed to consume this
	// client configuration
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	// +required
	ComponentName string `json:"componentName"`

	// componentNamespace is the namespace of the component that is supposed to consume this
	// client configuration
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +required
	ComponentNamespace string `json:"componentNamespace"`

	// clientID is the identifier of the OIDC client from the OIDC provider
	//
	// +kubebuilder:validation:MinLength=1
	// +required
	ClientID string `json:"clientID"`

	// clientSecret refers to a secret in the `openshift-config` namespace that
	// contains the client secret in the `clientSecret` key of the `.data` field
	ClientSecret SecretNameReference `json:"clientSecret"`

	// extraScopes is an optional set of scopes to request tokens with.
	//
	// +listType=set
	ExtraScopes []string `json:"extraScopes"`
}

type OIDCClientStatus struct {
	// componentName is the name of the component that will consume a client configuration.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	// +required
	ComponentName string `json:"componentName"`

	// componentNamespace is the namespace of the component that will consume a client configuration.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +required
	ComponentNamespace string `json:"componentNamespace"`

	// currentOIDCClients is a list of clients that the component is currently using.
	//
	// +listType=map
	// +listMapKey=issuerURL
	// +listMapKey=clientID
	CurrentOIDCClients []OIDCClientReference `json:"currentOIDCClients"`

	// consumingUsers is a slice of ServiceAccounts that need to have read
	// permission on the `clientSecret` secret.
	//
	// +kubebuilder:validation:MaxItems=5
	// +listType=set
	ConsumingUsers []ConsumingUser `json:"consumingUsers"`

	// conditions are used to communicate the state of the `oidcClients` entry.
	//
	// Supported conditions include Available, Degraded and Progressing.
	//
	// If Available is true, the component is successfully using the configured client.
	// If Degraded is true, that means something has gone wrong trying to handle the client configuration.
	// If Progressing is true, that means the component is taking some action related to the `oidcClients` entry.
	//
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type OIDCClientReference struct {
	// OIDCName refers to the `name` of the provider from `oidcProviders`
	//
	// +kubebuilder:validation:MinLength=1
	// +required
	OIDCProviderName string `json:"oidcProviderName"`

	// URL is the serving URL of the token issuer.
	// Must use the https:// scheme.
	//
	// +kubebuilder:validation:Pattern=`^https:\/\/[^\s]`
	// +required
	IssuerURL string `json:"issuerURL"`

	// clientID is the identifier of the OIDC client from the OIDC provider
	//
	// +kubebuilder:validation:MinLength=1
	// +required
	ClientID string `json:"clientID"`
}

// +kubebuilder:validation:XValidation:rule="has(self.prefixPolicy) && self.prefixPolicy == 'Prefix' ? (has(self.prefix) && size(self.prefix.prefixString) > 0) : !has(self.prefix)",message="prefix must be set if prefixPolicy is 'Prefix', but must remain unset otherwise"
type UsernameClaimMapping struct {
	TokenClaimMapping `json:",inline"`

	// prefixPolicy specifies how a prefix should apply.
	//
	// By default, claims other than `email` will be prefixed with the issuer URL to
	// prevent naming clashes with other plugins.
	//
	// Set to "NoPrefix" to disable prefixing.
	//
	// Example:
	//     (1) `prefix` is set to "myoidc:" and `claim` is set to "username".
	//         If the JWT claim `username` contains value `userA`, the resulting
	//         mapped value will be "myoidc:userA".
	//     (2) `prefix` is set to "myoidc:" and `claim` is set to "email". If the
	//         JWT `email` claim contains value "userA@myoidc.tld", the resulting
	//         mapped value will be "myoidc:userA@myoidc.tld".
	//     (3) `prefix` is unset, `issuerURL` is set to `https://myoidc.tld`,
	//         the JWT claims include "username":"userA" and "email":"userA@myoidc.tld",
	//         and `claim` is set to:
	//         (a) "username": the mapped value will be "https://myoidc.tld#userA"
	//         (b) "email": the mapped value will be "userA@myoidc.tld"
	//
	// +kubebuilder:validation:Enum={"", "NoPrefix", "Prefix"}
	PrefixPolicy UsernamePrefixPolicy `json:"prefixPolicy"`

	Prefix *UsernamePrefix `json:"prefix"`
}

type UsernamePrefixPolicy string

var (
	// NoOpinion let's the cluster assign prefixes.  If the username claim is email, there is no prefix
	// If the username claim is anything else, it is prefixed by the issuerURL
	NoOpinion UsernamePrefixPolicy = ""

	// NoPrefix means the username claim value will not have any  prefix
	NoPrefix UsernamePrefixPolicy = "NoPrefix"

	// Prefix means the prefix value must be specified.  It cannot be empty
	Prefix UsernamePrefixPolicy = "Prefix"
)

type UsernamePrefix struct {
	// +kubebuilder:validation:MinLength=1
	// +required
	PrefixString string `json:"prefixString"`
}

type PrefixedClaimMapping struct {
	TokenClaimMapping `json:",inline"`

	// prefix is a string to prefix the value from the token in the result of the
	// claim mapping.
	//
	// By default, no prefixing occurs.
	//
	// Example: if `prefix` is set to "myoidc:"" and the `claim` in JWT contains
	// an array of strings "a", "b" and  "c", the mapping will result in an
	// array of string "myoidc:a", "myoidc:b" and "myoidc:c".
	Prefix string `json:"prefix"`
}

type TokenValidationRuleType string

const (
	TokenValidationRuleTypeRequiredClaim = "RequiredClaim"
)

type TokenClaimValidationRule struct {
	// type sets the type of the validation rule
	//
	// +kubebuilder:validation:Enum={"RequiredClaim"}
	// +kubebuilder:default="RequiredClaim"
	Type TokenValidationRuleType `json:"type"`

	// requiredClaim allows configuring a required claim name and its expected
	// value
	RequiredClaim *TokenRequiredClaim `json:"requiredClaim"`
}

type TokenRequiredClaim struct {
	// claim is a name of a required claim. Only claims with string values are
	// supported.
	//
	// +kubebuilder:validation:MinLength=1
	// +required
	Claim string `json:"claim"`

	// requiredValue is the required value for the claim.
	//
	// +kubebuilder:validation:MinLength=1
	// +required
	RequiredValue string `json:"requiredValue"`
}
