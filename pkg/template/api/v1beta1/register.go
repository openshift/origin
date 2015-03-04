package v1beta1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta1",
		&Template{},
		&TemplateList{},
	)
	api.Scheme.AddKnownTypeWithName("v1beta1", "TemplateConfig", &Template{})
	api.Scheme.AddKnownTypeWithName("v1beta1", "RemoteTemplate", &RemoteTemplate{})
}

func (*Template) IsAnAPIObject()       {}
func (*TemplateList) IsAnAPIObject()   {}
func (*RemoteTemplate) IsAnAPIObject() {}
