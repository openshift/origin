package api

import (
	"k8s.io/kubernetes/pkg/runtime"
)

var Scheme = runtime.NewScheme()

func init() {
	Scheme.AddKnownTypes("",
		&MasterConfig{},
		&NodeConfig{},
		&SessionSecrets{},

		&IdentityProvider{},
		&BasicAuthPasswordIdentityProvider{},
		&AllowAllPasswordIdentityProvider{},
		&DenyAllPasswordIdentityProvider{},
		&HTPasswdPasswordIdentityProvider{},
		&LDAPPasswordIdentityProvider{},
		&RequestHeaderIdentityProvider{},
		&GitHubIdentityProvider{},
		&GoogleIdentityProvider{},
		&OpenIDIdentityProvider{},
		&GrantConfig{},

		&LDAPSyncConfig{},
	)
}

func (*IdentityProvider) IsAnAPIObject()                  {}
func (*BasicAuthPasswordIdentityProvider) IsAnAPIObject() {}
func (*AllowAllPasswordIdentityProvider) IsAnAPIObject()  {}
func (*DenyAllPasswordIdentityProvider) IsAnAPIObject()   {}
func (*HTPasswdPasswordIdentityProvider) IsAnAPIObject()  {}
func (*LDAPPasswordIdentityProvider) IsAnAPIObject()      {}
func (*RequestHeaderIdentityProvider) IsAnAPIObject()     {}
func (*GitHubIdentityProvider) IsAnAPIObject()            {}
func (*GoogleIdentityProvider) IsAnAPIObject()            {}
func (*OpenIDIdentityProvider) IsAnAPIObject()            {}
func (*GrantConfig) IsAnAPIObject()                       {}

func (*MasterConfig) IsAnAPIObject()   {}
func (*NodeConfig) IsAnAPIObject()     {}
func (*SessionSecrets) IsAnAPIObject() {}

func (*LDAPSyncConfig) IsAnAPIObject() {}
