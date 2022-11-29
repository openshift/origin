package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	GroupName     = "cloud.network.openshift.io"
	GroupVersion  = schema.GroupVersion{Group: GroupName, Version: "v1"}
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// Install is a function which adds this version to a scheme
	Install = SchemeBuilder.AddToScheme

	// SchemeGroupVersion generated code relies on this name
	// Deprecated
	SchemeGroupVersion = GroupVersion
	// AddToScheme exists solely to keep the old generators creating valid code
	// DEPRECATED
	AddToScheme = SchemeBuilder.AddToScheme
)

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&CloudPrivateIPConfig{},
		&CloudPrivateIPConfigList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
