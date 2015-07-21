package v1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1",
		&User{},
		&UserList{},
		&Identity{},
		&IdentityList{},
		&UserIdentityMapping{},
		&Group{},
		&GroupList{},
	)
}
