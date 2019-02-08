package externalipranger

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var DeprecatedGroupVersion = schema.GroupVersion{Group: "", Version: runtime.APIVersionInternal}

var (
	deprecatedSchemeBuilder = runtime.NewSchemeBuilder(addDeprecatedKnownTypes)
	DeprecatedInstallLegacy = deprecatedSchemeBuilder.AddToScheme
)

func addDeprecatedKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(DeprecatedGroupVersion,
		&ExternalIPRangerAdmissionConfig{},
	)
	return nil
}

var GroupVersion = schema.GroupVersion{Group: "network.openshift.io", Version: runtime.APIVersionInternal}

var (
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	Install       = deprecatedSchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&ExternalIPRangerAdmissionConfig{},
	)
	return nil
}
