package v1beta3

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta3",
		&Project{},
		&ProjectList{},
		&ProjectRequest{},
	)
}

func (*ProjectRequest) IsAnAPIObject() {}
func (*Project) IsAnAPIObject()        {}
func (*ProjectList) IsAnAPIObject()    {}
