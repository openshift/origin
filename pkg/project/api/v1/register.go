package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

const GroupName = ""

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: GroupName, Version: "v1"}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes, addConversionFuncs)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Project{},
		&ProjectList{},
		&ProjectRequest{},
	)
	return nil
}

func (obj *ProjectRequest) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
func (obj *Project) GetObjectKind() unversioned.ObjectKind        { return &obj.TypeMeta }
func (obj *ProjectList) GetObjectKind() unversioned.ObjectKind    { return &obj.TypeMeta }
