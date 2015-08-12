package docker10

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("1.0",
		&DockerImage{},
	)
}

func (*DockerImage) IsAnAPIObject() {}
