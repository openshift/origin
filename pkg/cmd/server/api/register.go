package api

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

// Scheme is the default instance of runtime.Scheme to which types in the Kubernetes API are already registered.
var Scheme = runtime.NewScheme()

const GroupName = ""

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: GroupName, Version: ""}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) unversioned.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns back a Group qualified GroupResource
func Resource(resource string) unversioned.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func init() {
	Scheme.AddKnownTypes(SchemeGroupVersion,
		&MasterConfig{},
		&NodeConfig{},
		&SessionSecrets{},

		&IdentityProvider{},
		&BasicAuthPasswordIdentityProvider{},
		&AllowAllPasswordIdentityProvider{},
		&DenyAllPasswordIdentityProvider{},
		&HTPasswdPasswordIdentityProvider{},
		&LDAPPasswordIdentityProvider{},
		&KeystonePasswordIdentityProvider{},
		&RequestHeaderIdentityProvider{},
		&GitHubIdentityProvider{},
		&GitLabIdentityProvider{},
		&GoogleIdentityProvider{},
		&OpenIDIdentityProvider{},
		&GrantConfig{},
		&AdmissionPluginConfig{},

		&LDAPSyncConfig{},
	)
}

func (*IdentityProvider) IsAnAPIObject()                  {}
func (*BasicAuthPasswordIdentityProvider) IsAnAPIObject() {}
func (*AllowAllPasswordIdentityProvider) IsAnAPIObject()  {}
func (*DenyAllPasswordIdentityProvider) IsAnAPIObject()   {}
func (*HTPasswdPasswordIdentityProvider) IsAnAPIObject()  {}
func (*LDAPPasswordIdentityProvider) IsAnAPIObject()      {}
func (*KeystonePasswordIdentityProvider) IsAnAPIObject()  {}
func (*RequestHeaderIdentityProvider) IsAnAPIObject()     {}
func (*GitHubIdentityProvider) IsAnAPIObject()            {}
func (*GitLabIdentityProvider) IsAnAPIObject()            {}
func (*GoogleIdentityProvider) IsAnAPIObject()            {}
func (*OpenIDIdentityProvider) IsAnAPIObject()            {}
func (*GrantConfig) IsAnAPIObject()                       {}
func (*AdmissionPluginConfig) IsAnAPIObject()             {}

func (*MasterConfig) IsAnAPIObject()   {}
func (*NodeConfig) IsAnAPIObject()     {}
func (*SessionSecrets) IsAnAPIObject() {}

func (*LDAPSyncConfig) IsAnAPIObject() {}
