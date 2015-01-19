package dockerpre012

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("pre012",
		&DockerImage{},
	)
}

func (*DockerImage) IsAnAPIObject() {}
