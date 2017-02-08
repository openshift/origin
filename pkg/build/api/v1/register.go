package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/watch/versioned"
)

const GroupName = ""

// LegacySchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: GroupName, Version: "v1"}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes, addConversionFuncs, addDefaultingFuncs)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Build{},
		&BuildList{},
		&BuildConfig{},
		&BuildConfigList{},
		&BuildLog{},
		&BuildRequest{},
		&BuildLogOptions{},
		&BinaryBuildRequestOptions{},

		&kapi.ListOptions{},
		&kapi.DeleteOptions{},
		&kapi.ExportOptions{},
	)
	versioned.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
