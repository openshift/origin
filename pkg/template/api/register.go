package api

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
		&Template{},
		&TemplateList{},
	)
}

func (*Template) IsAnAPIObject()     {}
func (*TemplateList) IsAnAPIObject() {}
