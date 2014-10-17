package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
		&Project{},
		&ProjectList{},
	)
}

func (*Project) IsAnAPIObject()     {}
func (*ProjectList) IsAnAPIObject() {}
