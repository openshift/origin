package v1beta2

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta2",
		&User{},
		&UserList{},
		&Identity{},
		&IdentityList{},
		&UserIdentityMapping{},
	)
}
