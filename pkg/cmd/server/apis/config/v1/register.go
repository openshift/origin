package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	coreinternalconversions "k8s.io/kubernetes/pkg/apis/core"

	buildv1 "github.com/openshift/api/build/v1"
	buildinternalconversions "github.com/openshift/origin/pkg/build/apis/build/v1"
	"github.com/openshift/origin/pkg/cmd/server/apis/config"
)

var (
	// Legacy is the 'v1' apiVersion of config
	LegacyGroupName          = ""
	LegacySchemeGroupVersion = schema.GroupVersion{Group: LegacyGroupName, Version: "v1"}
	legacySchemeBuilder      = runtime.NewSchemeBuilder(
		addKnownTypesToLegacy,
		config.InstallLegacy,
		coreinternalconversions.AddToScheme,
		buildinternalconversions.Install,

		addConversionFuncs,
		addDefaultingFuncs,
	)
	InstallLegacy = legacySchemeBuilder.AddToScheme

	externalLegacySchemeBuilder = runtime.NewSchemeBuilder(
		addKnownTypesToLegacy,
		buildv1.Install,
	)
	InstallLegacyExternal = externalLegacySchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypesToLegacy(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(LegacySchemeGroupVersion,
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
