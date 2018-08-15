package config

import (
	"github.com/openshift/origin/pkg/build/apis/build"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/kubernetes/pkg/apis/core"
)

var Scheme = runtime.NewScheme()

var Codecs = serializer.NewCodecFactory(Scheme)

const GroupName = ""

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: runtime.APIVersionInternal}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns back a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(
		addKnownTypes,
		core.AddToScheme,
		build.AddToScheme,
	)
	InstallLegacy = SchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	if err := scheme.AddIgnoredConversionType(&metav1.TypeMeta{}, &metav1.TypeMeta{}); err != nil {
		return err
	}
	scheme.AddKnownTypes(SchemeGroupVersion, KnownTypes...)
	return nil
}

var KnownTypes = []runtime.Object{
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

	&DefaultAdmissionConfig{},

	&BuildDefaultsConfig{},
	&BuildOverridesConfig{},
}
