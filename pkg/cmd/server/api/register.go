package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

var Scheme = runtime.NewScheme()

func init() {
	Scheme.AddKnownTypes("",
		&MasterConfig{},
		&NodeConfig{},

		&IdentityProviderUsage{},
		&IdentityProvider{},
		&BasicAuthPasswordIdentityProvider{},
		&AllowAllPasswordIdentityProvider{},
		&DenyAllPasswordIdentityProvider{},
		&HTPasswdPasswordIdentityProvider{},
		&RequestHeaderIdentityProvider{},
		&OAuthRedirectingIdentityProvider{},
		&GrantConfig{},
		&GoogleOAuthProvider{},
		&GitHubOAuthProvider{},
	)
}

func (*IdentityProviderUsage) IsAnAPIObject()             {}
func (*IdentityProvider) IsAnAPIObject()                  {}
func (*BasicAuthPasswordIdentityProvider) IsAnAPIObject() {}
func (*AllowAllPasswordIdentityProvider) IsAnAPIObject()  {}
func (*DenyAllPasswordIdentityProvider) IsAnAPIObject()   {}
func (*HTPasswdPasswordIdentityProvider) IsAnAPIObject()  {}
func (*RequestHeaderIdentityProvider) IsAnAPIObject()     {}
func (*OAuthRedirectingIdentityProvider) IsAnAPIObject()  {}
func (*GrantConfig) IsAnAPIObject()                       {}
func (*GoogleOAuthProvider) IsAnAPIObject()               {}
func (*GitHubOAuthProvider) IsAnAPIObject()               {}

func (*MasterConfig) IsAnAPIObject() {}
func (*NodeConfig) IsAnAPIObject()   {}
