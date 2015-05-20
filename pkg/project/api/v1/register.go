package v1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1",
		&Project{},
		&ProjectList{},
		&ProjectRequest{},
	)
}

func (*ProjectRequest) IsAnAPIObject() {}
func (*Project) IsAnAPIObject()        {}
func (*ProjectList) IsAnAPIObject()    {}
