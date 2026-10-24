package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const GroupName = "authentication.openshift.io"

var (
	GroupVersion  = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// Install adds this version to a scheme.
	Install = schemeBuilder.AddToScheme

	// SchemeGroupVersion is retained for generated-code compatibility.
	// Deprecated.
	SchemeGroupVersion = GroupVersion
	// AddToScheme is retained for generated-code compatibility.
	// Deprecated.
	AddToScheme = schemeBuilder.AddToScheme
)

// Resource is retained for generated-code compatibility.
// Deprecated.
func Resource(resource string) schema.GroupResource {
	return GroupVersion.WithResource(resource).GroupResource()
}

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion, &AuthenticationConfiguration{})
	return nil
}
