package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
		&Template{},
		&TemplateList{},
		&RemoteTemplate{},
	)
}

func (*Template) IsAnAPIObject()       {}
func (*TemplateList) IsAnAPIObject()   {}
func (*RemoteTemplate) IsAnAPIObject() {}
