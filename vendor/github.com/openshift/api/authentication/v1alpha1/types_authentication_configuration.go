package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*

The types in this file are heavily inspired by the existing
Kubernetes AuthenticationConfiguration type found at
https://github.com/kubernetes/kubernetes/blob/b2f73c0d6b427e2ab5ba225375aaefc0b9bc45b2/staging/src/k8s.io/apiserver/pkg/apis/apiserver/v1/types.go#L56

The API surface here intentionally aligns with this API because it is meant
to be a wrapper around that API with some customization to support unique
functionality that OpenShift needs.

*/

// AuthenticationConfiguration is the configuration-file API read by the
// oauth-apiserver when it runs in External OIDC mode.
//
// The cluster-authentication-operator is the only component that writes this
// configuration file. It generates the file from the user-facing
// config.openshift.io/v1, Kind=Authentication API. This type is not a CRD and
// must not be configured directly.
//
// The oauth-apiserver performs semantic validation when it reads this file. See
// https://github.com/openshift/oauth-apiserver/blob/97a820bd5412/pkg/externaloidc/apis/authentication/validation/validation.go.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type AuthenticationConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// jwt is a list of authenticator to authenticate Kubernetes users using
	// JWT compliant tokens. The authenticator will attempt to parse a raw ID token,
	// verify it's been signed by the configured issuer. The public key to verify the
	// signature is discovered from the issuer's public endpoint using OIDC discovery.
	// For an incoming token, each JWT authenticator will be attempted in
	// the order in which it is specified in this list.  Note however that
	// other authenticators may run before or after the JWT authenticators.
	// The specific position of JWT authenticators in relation to other
	// authenticators is neither defined nor stable across releases.  Since
	// each JWT authenticator must have a unique issuer URL, at most one
	// JWT authenticator will attempt to cryptographically validate the token.
	//
	// The minimum valid JWT payload must contain the following claims:
	// {
	//		"iss": "https://issuer.example.com",
	//		"aud": ["audience"],
	//		"exp": 1234567890,
	//		"<username claim>": "username"
	// }
	// +listType=atomic
	JWT []JWTAuthenticator `json:"jwt,omitempty"`
}

// JWTAuthenticator configures one JWT authenticator.
type JWTAuthenticator struct {
	// issuer contains the basic OIDC provider connection options.
	// +required
	Issuer *Issuer `json:"issuer,omitempty"`

	// claimValidationRules are rules that are applied to validate token claims to authenticate users.
	// +listType=atomic
	// +optional
	ClaimValidationRules []ClaimValidationRule `json:"claimValidationRules,omitempty"`

	// claimMappings points claims of a token to be treated as user attributes.
	// +required
	ClaimMappings *ClaimMappings `json:"claimMappings,omitempty"`

	// userValidationRules are rules that are applied to final user before completing authentication.
	// These allow invariants to be applied to incoming identities such as preventing the
	// use of the system: prefix that is commonly used by Kubernetes components.
	// The validation rules are logically ANDed together and must all return true for the validation to pass.
	// +listType=atomic
	// +optional
	UserValidationRules []UserValidationRule `json:"userValidationRules,omitempty"`

	// externalClaimsSources is an optional field that can be used to configure
	// sources, external to the token provided in a request, in which claims
	// should be fetched from and made available to the claim mapping process
	// that is used to build the identity of a token holder.
	// For example, fetching additional user metadata from an OIDC provider's UserInfo endpoint.
	// externalClaimsSources must not exceed 5 entries.
	// +listType=atomic
	// +optional
	ExternalClaimsSources []ExternalClaimsSource `json:"externalClaimsSources,omitempty"`
}

// Issuer provides the configuration for an external provider's specific settings.
type Issuer struct {
	// url points to the issuer URL in a format https://url or https://url/path.
	// This must match the "iss" claim in the presented JWT, and the issuer returned from discovery.
	// Same value as the --oidc-issuer-url flag.
	// Discovery information is fetched from "{url}/.well-known/openid-configuration" unless overridden by discoveryURL.
	// Required to be unique across all JWT authenticators.
	// Note that egress selection configuration is not used for this network connection.
	// +required
	URL string `json:"url"`

	// discoveryURL, if specified, overrides the URL used to fetch discovery
	// information instead of using "{url}/.well-known/openid-configuration".
	// The exact value specified is used, so "/.well-known/openid-configuration"
	// must be included in discoveryURL if needed.
	//
	// The "issuer" field in the fetched discovery information must match the "issuer.url" field
	// in the AuthenticationConfiguration and will be used to validate the "iss" claim in the presented JWT.
	// This is for scenarios where the well-known and jwks endpoints are hosted at a different
	// location than the issuer (such as locally in the cluster).
	//
	// Example:
	// A discovery url that is exposed using kubernetes service 'oidc' in namespace 'oidc-namespace'
	// and discovery information is available at '/.well-known/openid-configuration'.
	// discoveryURL: "https://oidc.oidc-namespace/.well-known/openid-configuration"
	// certificateAuthority is used to verify the TLS connection and the hostname on the leaf certificate
	// must be set to 'oidc.oidc-namespace'.
	//
	// curl https://oidc.oidc-namespace/.well-known/openid-configuration (.discoveryURL field)
	// {
	//     issuer: "https://oidc.example.com" (.url field)
	// }
	//
	// discoveryURL must be different from url.
	// Required to be unique across all JWT authenticators.
	// Note that egress selection configuration is not used for this network connection.
	// +optional
	DiscoveryURL string `json:"discoveryURL,omitempty"`

	// certificateAuthority contains PEM-encoded certificate authority certificates
	// used to validate the connection when fetching discovery information.
	// If unset, the system verifier is used.
	// Same value as the content of the file referenced by the --oidc-ca-file flag.
	// +optional
	CertificateAuthority string `json:"certificateAuthority,omitempty"`

	// audiences is the set of acceptable audiences the JWT must be issued to.
	// At least one of the entries must match the "aud" claim in presented JWTs.
	// Same value as the --oidc-client-id flag (though this field supports an array).
	// Required to be non-empty.
	// +listType=atomic
	// +required
	Audiences []string `json:"audiences"`

	// audienceMatchPolicy defines how the "audiences" field is used to match the "aud" claim in the presented JWT.
	// Allowed values are:
	// 1. "MatchAny" when multiple audiences are specified and
	// 2. empty (or unset) or "MatchAny" when a single audience is specified.
	//
	// - MatchAny: the "aud" claim in the presented JWT must match at least one of the entries in the "audiences" field.
	// For example, if "audiences" is ["foo", "bar"], the "aud" claim in the presented JWT must contain either "foo" or "bar" (and may contain both).
	//
	// - "": The match policy can be empty (or unset) when a single audience is specified in the "audiences" field. The "aud" claim in the presented JWT must contain the single audience (and may contain others).
	//
	// For more nuanced audience validation, use claimValidationRules.
	//   example: claimValidationRule[].expression: 'sets.equivalent(claims.aud, ["bar", "foo", "baz"])' to require an exact match.
	// +optional
	AudienceMatchPolicy AudienceMatchPolicyType `json:"audienceMatchPolicy,omitempty"`
}

// AudienceMatchPolicyType is a set of valid values for issuer.audienceMatchPolicy
type AudienceMatchPolicyType string

// Valid types for AudienceMatchPolicyType
const (
	// MatchAny means the "aud" claim in the presented JWT must match at least one of the entries in the "audiences" field.
	AudienceMatchPolicyMatchAny AudienceMatchPolicyType = "MatchAny"
)

// ClaimValidationRule provides the configuration for a single claim validation rule.
type ClaimValidationRule struct {
	// claim is the name of a required claim.
	// Same as --oidc-required-claim flag.
	// Only string claim keys are supported.
	// Mutually exclusive with expression and message.
	// +optional
	Claim string `json:"claim,omitempty"`
	// requiredValue is the value of a required claim.
	// Same as --oidc-required-claim flag.
	// Only string claim values are supported.
	// If claim is set and requiredValue is not set, the claim must be present with a value set to the empty string.
	// Mutually exclusive with expression and message.
	// +optional
	RequiredValue string `json:"requiredValue,omitempty"`

	// expression represents the expression which will be evaluated by CEL.
	// Must produce a boolean.
	//
	// CEL expressions have access to the contents of the token claims, organized into CEL variable:
	// - 'claims' is a map of claim names to claim values.
	//   For example, a variable named 'sub' can be accessed as 'claims.sub'.
	//   Nested claims can be accessed using dot notation, e.g. 'claims.foo.bar'.
	// Must return true for the validation to pass.
	//
	// Documentation on CEL: https://kubernetes.io/docs/reference/using-api/cel/
	//
	// Mutually exclusive with claim and requiredValue.
	// +optional
	Expression string `json:"expression,omitempty"`
	// message customizes the returned error message when expression returns false.
	// message is a literal string.
	// Mutually exclusive with claim and requiredValue.
	// +optional
	Message string `json:"message,omitempty"`
}

// ClaimMappings provides the configuration for claim mapping
type ClaimMappings struct {
	// username represents an option for the username attribute.
	// The claim's value must be a singular string.
	// Same as the --oidc-username-claim and --oidc-username-prefix flags.
	// If username.expression is set, the expression must produce a string value.
	// If username.expression uses 'claims.email', then 'claims.email_verified' must be used in
	// username.expression or extra[*].valueExpression or claimValidationRules[*].expression.
	// An example claim validation rule expression that matches the validation automatically
	// applied when username.claim is set to 'email' is 'claims.?email_verified.orValue(true) == true'. By explicitly comparing
	// the value to true, we let type-checking see the result will be a boolean, and to make sure a non-boolean email_verified
	// claim will be caught at runtime.
	//
	// In the flag based approach, the --oidc-username-claim and --oidc-username-prefix are optional. If --oidc-username-claim is not set,
	// the default value is "sub". For the authentication config, there is no defaulting for claim or prefix. The claim and prefix must be set explicitly.
	// For claim, if --oidc-username-claim was not set with legacy flag approach, configure username.claim="sub" in the authentication config.
	// For prefix:
	//     (1) --oidc-username-prefix="-", no prefix was added to the username. For the same behavior using authentication config,
	//         set username.prefix=""
	//     (2) --oidc-username-prefix="" and  --oidc-username-claim != "email", prefix was "<value of --oidc-issuer-url>#". For the same
	//         behavior using authentication config, set username.prefix="<value of issuer.url>#"
	//     (3) --oidc-username-prefix="<value>". For the same behavior using authentication config, set username.prefix="<value>"
	// +required
	Username PrefixedClaimOrExpression `json:"username"`
	// groups represents an option for the groups attribute.
	// The claim's value must be a string or string array claim.
	// If groups.claim is set, the prefix must be specified (and can be the empty string).
	// If groups.expression is set, the expression must produce a string or string array value.
	//  "", [], and null values are treated as the group mapping not being present.
	// +optional
	Groups PrefixedClaimOrExpression `json:"groups,omitempty"`

	// uid represents an option for the uid attribute.
	// Claim must be a singular string claim.
	// If uid.expression is set, the expression must produce a string value.
	// +optional
	UID ClaimOrExpression `json:"uid"`

	// extra represents an option for the extra attribute.
	// expression must produce a string or string array value.
	// If the value is empty, the extra mapping will not be present.
	//
	// hard-coded extra key/value
	// - key: "foo"
	//   valueExpression: "'bar'"
	// This will result in an extra attribute - foo: ["bar"]
	//
	// hard-coded key, value copying claim value
	// - key: "foo"
	//   valueExpression: "claims.some_claim"
	// This will result in an extra attribute - foo: [value of some_claim]
	//
	// hard-coded key, value derived from claim value
	// - key: "admin"
	//   valueExpression: '(has(claims.is_admin) && claims.is_admin) ? "true":""'
	// This will result in:
	//  - if is_admin claim is present and true, extra attribute - admin: ["true"]
	//  - if is_admin claim is present and false or is_admin claim is not present, no extra attribute will be added
	//
	// +listType=atomic
	// +optional
	Extra []ExtraMapping `json:"extra,omitempty"`
}

// PrefixedClaimOrExpression provides the configuration for a single prefixed claim or expression.
type PrefixedClaimOrExpression struct {
	// claim is the JWT claim to use.
	// Mutually exclusive with expression.
	// +optional
	Claim string `json:"claim,omitempty"`
	// prefix is prepended to claim's value to prevent clashes with existing names.
	// prefix needs to be set if claim is set and can be the empty string.
	// Mutually exclusive with expression.
	// +optional
	Prefix *string `json:"prefix,omitempty"`

	// expression represents the expression which will be evaluated by CEL.
	//
	// CEL expressions have access to the contents of the token claims, organized into CEL variable:
	// - 'claims' is a map of claim names to claim values.
	//   For example, a variable named 'sub' can be accessed as 'claims.sub'.
	//   Nested claims can be accessed using dot notation, e.g. 'claims.foo.bar'.
	//
	// Documentation on CEL: https://kubernetes.io/docs/reference/using-api/cel/
	//
	// Mutually exclusive with claim and prefix.
	// +optional
	Expression string `json:"expression,omitempty"`
}

// ClaimOrExpression provides the configuration for a single claim or expression.
type ClaimOrExpression struct {
	// claim is the JWT claim to use.
	// Either claim or expression must be set.
	// Mutually exclusive with expression.
	// +optional
	Claim string `json:"claim,omitempty"`

	// expression represents the expression which will be evaluated by CEL.
	//
	// CEL expressions have access to the contents of the token claims, organized into CEL variable:
	// - 'claims' is a map of claim names to claim values.
	//   For example, a variable named 'sub' can be accessed as 'claims.sub'.
	//   Nested claims can be accessed using dot notation, e.g. 'claims.foo.bar'.
	//
	// Documentation on CEL: https://kubernetes.io/docs/reference/using-api/cel/
	//
	// Mutually exclusive with claim.
	// +optional
	Expression string `json:"expression,omitempty"`
}

// ExtraMapping provides the configuration for a single extra mapping.
type ExtraMapping struct {
	// key is a string to use as the extra attribute key.
	// key must be a domain-prefix path (e.g. example.org/foo). All characters before the first "/" must be a valid
	// subdomain as defined by RFC 1123. All characters trailing the first "/" must
	// be valid HTTP Path characters as defined by RFC 3986.
	// key must be lowercase.
	// Required to be unique.
	// +required
	Key string `json:"key"`

	// valueExpression is a CEL expression to extract extra attribute value.
	// valueExpression must produce a string or string array value.
	// "", [], and null values are treated as the extra mapping not being present.
	// Empty string values contained within a string array are filtered out.
	//
	// CEL expressions have access to the contents of the token claims, organized into CEL variable:
	// - 'claims' is a map of claim names to claim values.
	//   For example, a variable named 'sub' can be accessed as 'claims.sub'.
	//   Nested claims can be accessed using dot notation, e.g. 'claims.foo.bar'.
	//
	// Documentation on CEL: https://kubernetes.io/docs/reference/using-api/cel/
	//
	// +required
	ValueExpression string `json:"valueExpression"`
}

// UserValidationRule provides the configuration for a single user info validation rule.
type UserValidationRule struct {
	// expression represents the expression which will be evaluated by CEL.
	// Must return true for the validation to pass.
	//
	// CEL expressions have access to the contents of UserInfo, organized into CEL variable:
	// - 'user' - authentication.k8s.io/v1, Kind=UserInfo object
	//    Refer to https://github.com/kubernetes/api/blob/release-1.28/authentication/v1/types.go#L105-L122 for the definition.
	//    API documentation: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#userinfo-v1-authentication-k8s-io
	//
	// Documentation on CEL: https://kubernetes.io/docs/reference/using-api/cel/
	//
	// +required
	Expression string `json:"expression"`

	// message customizes the returned error message when rule returns false.
	// message is a literal string.
	// +optional
	Message string `json:"message,omitempty"`
}

// ExternalClaimsSource provides the configuration for a single external claim source.
type ExternalClaimsSource struct {
	// authentication is an optional field that configures how the apiserver authenticates with an external claims source.
	// When not specified, anonymous authentication is used.
	// +optional
	Authentication *Authentication `json:"authentication,omitempty"`
	// tls is an optional field that configures the http client TLS
	// settings when fetching external claims from this source.
	// At least one subfield must be set when this field is specified.
	// +optional
	TLS *TLS `json:"tls,omitempty"`
	// url is a required configuration of the URL
	// for which the external claims are located.
	// +required
	URL *SourceURL `json:"url,omitempty"`
	// mappings is a required list of the claim
	// and response handling expression pairs
	// that produces the claims from the external source.
	//
	// mappings must have at least 1 entry and must not exceed 16 entries.
	// Entries must have a unique name across all external claim sources.
	//
	// WARNING: claims sourced using these mappings will override any claims
	// that exist within the token during the claim-to-identity mapping
	// process. Use caution when sourcing external claims to avoid unintentionally
	// overriding token claims. To help guard against this, sourcing
	// external claims can have guard conditions defined in the 'conditions'
	// field.
	//
	// +listType=atomic
	// +required
	Mappings []SourcedClaimMapping `json:"mappings,omitempty"`
	// conditions is an optional list of conditions in
	// which claims should attempt to be fetched from this
	// external source.
	// When omitted or empty, claims are always attempted to be fetched
	// from this external source.
	// When specified, all conditions must evaluate to 'true'
	// before claims are attempted to be fetched from this external source.
	// conditions must not exceed 16 entries.
	// Entries must have unique expressions.
	// +listType=atomic
	// +optional
	Conditions []ExternalSourceCondition `json:"conditions,omitempty"`
}

// TLS configures the TLS options that the apiserver uses as a client
// when making a request to the external claim source.
// At least one field must be set when specified.
type TLS struct {
	// certificateAuthority is an optional field that configures the certificate authority
	// used to validate TLS connections with the external claims source.
	// Must not be empty and must be a valid PEM-encoded certificate.
	// +optional
	CertificateAuthority *string `json:"certificateAuthority,omitempty"`
}

// Authentication configures how the apiserver should attempt to authenticate
// with an external claims source.
type Authentication struct {
	// type is a required field that sets the type of
	// authentication method used by the authenticator
	// when fetching external claims.
	//
	// Allowed values are 'RequestProvidedToken' and 'ClientCredential'.
	//
	// When set to 'RequestProvidedToken', the authenticator will
	// use the token provided to the kube-apiserver as part of the
	// request to authenticate with the external claims source.
	//
	// When set to 'ClientCredential', the authenticator will
	// use the configured client-id, client-secret, and token endpoint
	// to fetch an access token using the OAuth2 client credentials grant
	// flow. The fetched access token will then be used to authenticate
	// with the external claims source.
	// +required
	Type *AuthenticationType `json:"type,omitempty"`

	// clientCredential configures the client credentials
	// and token endpoint to use to get an access token.
	// This field must be set when type is ClientCredential.
	// This field must not be set when type is not ClientCredential.
	// +optional
	ClientCredential *ClientCredentialConfig `json:"clientCredential,omitempty"`
}

// AuthenticationType is the type of authentication that should be used
// when fetching claims from an external source.
type AuthenticationType string

const (
	// AuthenticationTypeRequestProvidedToken is an AuthenticationType
	// that represents that the token being evaluated for authentication
	// should be used for authenticating with the external claims source.
	// This is useful for scenarios where a token has multiple audiences
	// and scopes so that it can be used to access both the cluster and
	// the UserInfo endpoint that contains additional information about the
	// user not present in the token.
	AuthenticationTypeRequestProvidedToken AuthenticationType = "RequestProvidedToken"

	// AuthenticationTypeClientCredential is an AuthenticationType
	// that represents that the authenticator should use the OAuth2
	// client credentials grant flow to obtain an access token for
	// authenticating with the external claims source.
	// This is useful for scenarios such as fetching user information
	// from Microsoft's Graph API where a separate client credential
	// is needed to access the API.
	AuthenticationTypeClientCredential AuthenticationType = "ClientCredential"
)

// ClientCredentialConfig configures the client credentials and token endpoint
// to use to get an access token via the OAuth2 client credentials grant flow.
type ClientCredentialConfig struct {
	// clientID is the client identifier to use during the OAuth2 client credentials flow.
	// clientID must not be an empty string ("").
	// clientID must only contain printable ASCII characters.
	// +required
	ClientID string `json:"clientID,omitempty"`

	// clientSecret is the client secret to use during the OAuth2 client credentials flow.
	// clientSecret is the literal string value of the client secret.
	// clientSecret must not be an empty string ("").
	// clientSecret must only contain printable ASCII characters.
	// +required
	ClientSecret string `json:"clientSecret,omitempty"`

	// tokenEndpoint is a required URL to query for an access token using
	// the client credential OAuth2 flow.
	// tokenEndpoint must not be an empty string ("").
	// tokenEndpoint must be a valid HTTPS URL.
	// tokenEndpoint must have a host and a path.
	// tokenEndpoint must not contain query parameters, fragments,
	// or user information (e.g., "user:password@host").
	// +required
	TokenEndpoint string `json:"tokenEndpoint,omitempty"`

	// scopes is an optional list of OAuth2 scopes to request when obtaining
	// an access token. If not specified, the token endpoint's default scopes
	// will be used. Each scope must not be an empty string ("").
	// +listType=atomic
	// +optional
	Scopes []string `json:"scopes,omitempty"`

	// tls is an optional field that configures the http client TLS
	// settings when fetching an access token for this source.
	// At least one subfield must be set when this field is specified.
	// +optional
	TLS *TLS `json:"tls,omitempty"`
}

// SourceURL configures the options used to build the URL that is queried for external claims.
type SourceURL struct {
	// hostname is a required hostname for which the external claims are located.
	// It must be a valid DNS subdomain name as per RFC1123.
	// This means that it must start and end with a lowercase alphanumeric character,
	// must only consist of lowercase alphanumeric characters, '-', and '.'.
	// hostname must not be an empty string ("") and must not exceed 253 characters in length.
	// hostname may optionally specify a port in the format ':{port}'.
	// +required
	Hostname *string `json:"hostname,omitempty"`
	// pathExpression is a required CEL expression that returns a list
	// of string values used to construct the URL path.
	// Claims from the token used for the request to the kube-apiserver
	// are made available via the `claims` variable.
	// expression must not be an empty string ("").
	// +required
	PathExpression *string `json:"pathExpression,omitempty"`
}

// SourcedClaimMapping configures the mapping behavior for a single external claim
// from the response the apiserver received from the external claim source.
type SourcedClaimMapping struct {
	// name is a required name of the claim that
	// will be produced and made available during
	// the claim-to-identity mapping process.
	// name must consist of only lowercase alpha characters and underscores ('_').
	// name must not be an empty string ("") and must not exceed 256 characters in length.
	// +required
	Name *string `json:"name,omitempty"`

	// expression is a required CEL expression that
	// will produce a value to be assigned to the claim.
	// The full response body from the request to the
	// external claim source is provided via the
	// `response` variable.
	// expression must not be an empty string ("").
	// +required
	Expression *string `json:"expression,omitempty"`
}

// ExternalSourceCondition configures a singular condition
// that must return true before the external source is queried
// to retrieve external claims.
type ExternalSourceCondition struct {
	// expression is a required CEL expression that
	// is used to determine whether or not an external
	// source should be used to fetch external claims.
	// The expression must return a boolean value,
	// where true means that the source should be consulted
	// and false means that it should not.
	// Claims from the token used for the request to the kube-apiserver
	// are made available via the `claims` variable.
	// expression must not be an empty string ("").
	// +required
	Expression *string `json:"expression,omitempty"`
}
