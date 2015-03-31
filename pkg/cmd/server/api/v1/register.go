package v1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/openshift/origin/pkg/cmd/server/api"
)

var Codec = runtime.CodecFor(api.Scheme, "v1")

func init() {
	api.Scheme.AddKnownTypes("v1",
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
