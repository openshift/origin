package api

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: runtime.APIVersionInternal}

// Adds the list of known types to api.Scheme.
func AddToScheme(scheme *runtime.Scheme) {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ClusterResourceOverrideConfig{},
	)
}

func (obj *ClusterResourceOverrideConfig) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
