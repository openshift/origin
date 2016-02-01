package api

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/runtime/serializer"
)

var Scheme = runtime.NewScheme()

var Codecs = serializer.NewCodecFactory(Scheme)

const GroupName = ""

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: GroupName, Version: runtime.APIVersionInternal}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) unversioned.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns back a Group qualified GroupResource
func Resource(resource string) unversioned.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func AddToScheme(scheme *runtime.Scheme) {
	// Add the API to Scheme.
	addKnownTypes(scheme)
}

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) {
	if err := scheme.AddIgnoredConversionType(&unversioned.TypeMeta{}, &unversioned.TypeMeta{}); err != nil {
		panic(err)
	}
	scheme.AddKnownTypes(SchemeGroupVersion,
		&MasterConfig{},
		&NodeConfig{},
		&SessionSecrets{},

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

		&LDAPSyncConfig{},
	)
}

func (obj *LDAPSyncConfig) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }

func (obj *OpenIDIdentityProvider) GetObjectKind() unversioned.ObjectKind        { return &obj.TypeMeta }
func (obj *GoogleIdentityProvider) GetObjectKind() unversioned.ObjectKind        { return &obj.TypeMeta }
func (obj *GitLabIdentityProvider) GetObjectKind() unversioned.ObjectKind        { return &obj.TypeMeta }
func (obj *GitHubIdentityProvider) GetObjectKind() unversioned.ObjectKind        { return &obj.TypeMeta }
func (obj *RequestHeaderIdentityProvider) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
func (obj *KeystonePasswordIdentityProvider) GetObjectKind() unversioned.ObjectKind {
	return &obj.TypeMeta
}
func (obj *LDAPPasswordIdentityProvider) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
func (obj *HTPasswdPasswordIdentityProvider) GetObjectKind() unversioned.ObjectKind {
	return &obj.TypeMeta
}
func (obj *DenyAllPasswordIdentityProvider) GetObjectKind() unversioned.ObjectKind {
	return &obj.TypeMeta
}
func (obj *AllowAllPasswordIdentityProvider) GetObjectKind() unversioned.ObjectKind {
	return &obj.TypeMeta
}
func (obj *BasicAuthPasswordIdentityProvider) GetObjectKind() unversioned.ObjectKind {
	return &obj.TypeMeta
}

func (obj *SessionSecrets) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
func (obj *NodeConfig) GetObjectKind() unversioned.ObjectKind     { return &obj.TypeMeta }
func (obj *MasterConfig) GetObjectKind() unversioned.ObjectKind   { return &obj.TypeMeta }
