package dockerpre012

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("pre012",
		&DockerImage{},
	)
}

func (*DockerImage) IsAnAPIObject() {}
