package v1beta3

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta3",
		&Project{},
		&ProjectList{},
	)
}

func (*Project) IsAnAPIObject()     {}
func (*ProjectList) IsAnAPIObject() {}
