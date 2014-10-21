package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
		&User{},
		&Identity{},
		&UserIdentityMapping{},
	)
}
