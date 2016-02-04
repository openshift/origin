package v1beta3

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: "v1beta3"}

// Codec encodes internal objects to the v1beta3 scheme
var Codec = runtime.CodecFor(api.Scheme, SchemeGroupVersion.String())

func init() {
	api.Scheme.AddKnownTypes(SchemeGroupVersion,
		&Template{},
		&TemplateList{},
	)

	api.Scheme.AddKnownTypeWithName(SchemeGroupVersion.WithKind("TemplateConfig"), &Template{})
	api.Scheme.AddKnownTypeWithName(SchemeGroupVersion.WithKind("ProcessedTemplate"), &Template{})
}

func (*Template) IsAnAPIObject()     {}
func (*TemplateList) IsAnAPIObject() {}
