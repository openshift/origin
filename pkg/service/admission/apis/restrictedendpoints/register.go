package restrictedendpoints

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var DeprecatedSchemeGroupVersion = schema.GroupVersion{Group: "", Version: runtime.APIVersionInternal}

var (
	deprecatedSchemeBuilder = runtime.NewSchemeBuilder(addDeprecatedKnownTypes)
	DeprecatedInstall       = deprecatedSchemeBuilder.AddToScheme
)

func addDeprecatedKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(DeprecatedSchemeGroupVersion,
		&RestrictedEndpointsAdmissionConfig{},
	)
	return nil
}

var GroupVersion = schema.GroupVersion{Group: "network.openshift.io", Version: runtime.APIVersionInternal}

var (
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	Install       = schemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&RestrictedEndpointsAdmissionConfig{},
	)
	return nil
}
