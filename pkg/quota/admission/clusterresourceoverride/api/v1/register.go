package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: "v1"}

// Adds the list of known types to api.Scheme.
func AddToScheme(scheme *runtime.Scheme) {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ClusterResourceOverrideConfig{},
	)
}

func (obj *ClusterResourceOverrideConfig) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
