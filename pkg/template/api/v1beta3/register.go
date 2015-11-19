package v1beta3

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta3",
		&Template{},
		&TemplateList{},
	)
	api.Scheme.AddKnownTypeWithName("v1beta3", "TemplateConfig", &Template{})
	api.Scheme.AddKnownTypeWithName("v1beta3", "ProcessedTemplate", &Template{})
}

func (*Template) IsAnAPIObject()     {}
func (*TemplateList) IsAnAPIObject() {}
