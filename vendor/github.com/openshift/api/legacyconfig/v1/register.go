package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// Legacy is the 'v1' apiVersion of config
	LegacyGroupName          = ""
	GroupVersion             = schema.GroupVersion{Group: LegacyGroupName, Version: "v1"}
	LegacySchemeGroupVersion = GroupVersion
	legacySchemeBuilder      = runtime.NewSchemeBuilder(
		addKnownTypesToLegacy,
	)
	InstallLegacy = legacySchemeBuilder.AddToScheme
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
