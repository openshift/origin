package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: "v1"}

func AddToScheme(scheme *runtime.Scheme) {
	addKnownTypes(scheme)
	addDefaultingFuncs(scheme)
}

func addKnownTypes(scheme *runtime.Scheme) {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&PodNodeConstraintsConfig{},
	)
}

func (obj *PodNodeConstraintsConfig) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
