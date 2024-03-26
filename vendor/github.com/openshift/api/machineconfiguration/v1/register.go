package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupName is the group name of this api
	GroupName = "machineconfiguration.openshift.io"
	// GroupVersion is the version of this api group
	GroupVersion  = schema.GroupVersion{Group: GroupName, Version: "v1"}
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// Install is a function which adds this version to a scheme
	Install = schemeBuilder.AddToScheme

	// SchemeGroupVersion is DEPRECATED
	SchemeGroupVersion = GroupVersion
	// AddToScheme is DEPRECATED
	AddToScheme = Install
)

// addKnownTypes adds types to API group
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&ContainerRuntimeConfig{},
		&ContainerRuntimeConfigList{},
		&ControllerConfig{},
		&ControllerConfigList{},
		&KubeletConfig{},
		&KubeletConfigList{},
		&MachineConfig{},
		&MachineConfigList{},
		&MachineConfigPool{},
		&MachineConfigPoolList{},
	)

	metav1.AddToGroupVersion(scheme, GroupVersion)

	return nil
}

// Resource is used to validate existence of a resource in this API group
func Resource(resource string) schema.GroupResource {
	return schema.GroupResource{Group: GroupName, Resource: resource}
}

// Kind is used to validate existence of a resource kind in this API group
func Kind(kind string) schema.GroupKind {
	return schema.GroupKind{Group: GroupName, Kind: kind}
}
