package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	GroupName     = "machineconfiguration.openshift.io"
	GroupVersion  = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// Install is a function which adds this version to a scheme
	Install = schemeBuilder.AddToScheme

	// SchemeGroupVersion generated code relies on this name
	// Deprecated
	SchemeGroupVersion = GroupVersion
	// AddToScheme exists solely to keep the old generators creating valid code
	// DEPRECATED
	AddToScheme = schemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&MachineConfigNode{},
		&MachineConfigNodeList{},
		&PinnedImageSet{},
		&PinnedImageSetList{},
		&OSImageStream{},
		&OSImageStreamList{},
		&InternalReleaseImage{},
		&InternalReleaseImageList{},
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}

// Resource generated code relies on this being here, but it logically belongs to the group
// DEPRECATED
func Resource(resource string) schema.GroupResource {
	return schema.GroupResource{Group: GroupName, Resource: resource}
}

// Kind is used to validate existence of a resource kind in this API group
func Kind(kind string) schema.GroupKind {
	return schema.GroupKind{Group: GroupName, Kind: kind}
}
