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
		&Template{},
		&TemplateList{},
	)

	scheme.AddKnownTypeWithName(SchemeGroupVersion.WithKind("TemplateConfig"), &Template{})
	scheme.AddKnownTypeWithName(SchemeGroupVersion.WithKind("ProcessedTemplate"), &Template{})

	return nil
}

func (obj *Template) GetObjectKind() unversioned.ObjectKind     { return &obj.TypeMeta }
func (obj *TemplateList) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
