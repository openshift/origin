package docker10

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("1.0",
		&DockerImage{},
	)
}

func (*DockerImage) IsAnAPIObject() {}
