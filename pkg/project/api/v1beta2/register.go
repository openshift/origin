package v1beta2

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta2",
		&Project{},
		&ProjectList{},
	)
}

func (*Project) IsAnAPIObject()     {}
func (*ProjectList) IsAnAPIObject() {}
