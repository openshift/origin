package v1beta2

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta2",
		&Template{},
		&TemplateList{},
	)
	api.Scheme.AddKnownTypeWithName("v1beta2", "TemplateConfig", &Template{})
}

func (*Template) IsAnAPIObject()     {}
func (*TemplateList) IsAnAPIObject() {}
