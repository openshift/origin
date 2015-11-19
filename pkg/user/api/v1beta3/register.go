package v1beta3

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta3",
		&User{},
		&UserList{},
		&Identity{},
		&IdentityList{},
		&UserIdentityMapping{},
		&Group{},
		&GroupList{},
	)
}
