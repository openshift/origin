// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1

// OpenIDIdentityProviderApplyConfiguration represents a declarative configuration of the OpenIDIdentityProvider type for use
// with apply.
type OpenIDIdentityProviderApplyConfiguration struct {
	ClientID                 *string                                   `json:"clientID,omitempty"`
	ClientSecret             *SecretNameReferenceApplyConfiguration    `json:"clientSecret,omitempty"`
	CA                       *ConfigMapNameReferenceApplyConfiguration `json:"ca,omitempty"`
	ExtraScopes              []string                                  `json:"extraScopes,omitempty"`
	ExtraAuthorizeParameters map[string]string                         `json:"extraAuthorizeParameters,omitempty"`
	Issuer                   *string                                   `json:"issuer,omitempty"`
	Claims                   *OpenIDClaimsApplyConfiguration           `json:"claims,omitempty"`
}

// OpenIDIdentityProviderApplyConfiguration constructs a declarative configuration of the OpenIDIdentityProvider type for use with
// apply.
func OpenIDIdentityProvider() *OpenIDIdentityProviderApplyConfiguration {
	return &OpenIDIdentityProviderApplyConfiguration{}
}

// WithClientID sets the ClientID field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ClientID field is set to the value of the last call.
func (b *OpenIDIdentityProviderApplyConfiguration) WithClientID(value string) *OpenIDIdentityProviderApplyConfiguration {
	b.ClientID = &value
	return b
}

// WithClientSecret sets the ClientSecret field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ClientSecret field is set to the value of the last call.
func (b *OpenIDIdentityProviderApplyConfiguration) WithClientSecret(value *SecretNameReferenceApplyConfiguration) *OpenIDIdentityProviderApplyConfiguration {
	b.ClientSecret = value
	return b
}

// WithCA sets the CA field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the CA field is set to the value of the last call.
func (b *OpenIDIdentityProviderApplyConfiguration) WithCA(value *ConfigMapNameReferenceApplyConfiguration) *OpenIDIdentityProviderApplyConfiguration {
	b.CA = value
	return b
}

// WithExtraScopes adds the given value to the ExtraScopes field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the ExtraScopes field.
func (b *OpenIDIdentityProviderApplyConfiguration) WithExtraScopes(values ...string) *OpenIDIdentityProviderApplyConfiguration {
	for i := range values {
		b.ExtraScopes = append(b.ExtraScopes, values[i])
	}
	return b
}

// WithExtraAuthorizeParameters puts the entries into the ExtraAuthorizeParameters field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, the entries provided by each call will be put on the ExtraAuthorizeParameters field,
// overwriting an existing map entries in ExtraAuthorizeParameters field with the same key.
func (b *OpenIDIdentityProviderApplyConfiguration) WithExtraAuthorizeParameters(entries map[string]string) *OpenIDIdentityProviderApplyConfiguration {
	if b.ExtraAuthorizeParameters == nil && len(entries) > 0 {
		b.ExtraAuthorizeParameters = make(map[string]string, len(entries))
	}
	for k, v := range entries {
		b.ExtraAuthorizeParameters[k] = v
	}
	return b
}

// WithIssuer sets the Issuer field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Issuer field is set to the value of the last call.
func (b *OpenIDIdentityProviderApplyConfiguration) WithIssuer(value string) *OpenIDIdentityProviderApplyConfiguration {
	b.Issuer = &value
	return b
}

// WithClaims sets the Claims field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Claims field is set to the value of the last call.
func (b *OpenIDIdentityProviderApplyConfiguration) WithClaims(value *OpenIDClaimsApplyConfiguration) *OpenIDIdentityProviderApplyConfiguration {
	b.Claims = value
	return b
}
