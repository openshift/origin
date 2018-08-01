package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	coreinternalconversions "k8s.io/kubernetes/pkg/apis/core"

	buildinternalconversions "github.com/openshift/origin/pkg/build/apis/build/v1"
	"github.com/openshift/origin/pkg/cmd/server/apis/config"
)

const GroupName = ""

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1"}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(
		addKnownTypes,
		config.InstallLegacy,
		coreinternalconversions.AddToScheme,
		buildinternalconversions.Install,

		addConversionFuncs,
		addDefaultingFuncs,
	)
	InstallLegacy = SchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
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

		&DefaultAdmissionConfig{},

		&BuildDefaultsConfig{},
		&BuildOverridesConfig{},
	)
	return nil
}
