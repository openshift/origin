package api

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
		&User{},
		&UserList{},
		&Identity{},
		&IdentityList{},
		&UserIdentityMapping{},
		&Group{},
		&GroupList{},
	)
}
