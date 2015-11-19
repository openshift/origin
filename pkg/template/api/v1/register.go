package v1

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1",
		&Template{},
		&TemplateList{},
	)
	api.Scheme.AddKnownTypeWithName("v1", "TemplateConfig", &Template{})
	api.Scheme.AddKnownTypeWithName("v1", "ProcessedTemplate", &Template{})
}

func (*Template) IsAnAPIObject()     {}
func (*TemplateList) IsAnAPIObject() {}
