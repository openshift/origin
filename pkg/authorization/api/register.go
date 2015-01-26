package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
		&Role{},
		&RoleBinding{},
		&Policy{},
		&PolicyBinding{},
		&SubjectAccessReview{},
		&PolicyList{},
		&PolicyBindingList{},
	)
}

func (*Role) IsAnAPIObject()                {}
func (*Policy) IsAnAPIObject()              {}
func (*PolicyBinding) IsAnAPIObject()       {}
func (*RoleBinding) IsAnAPIObject()         {}
func (*SubjectAccessReview) IsAnAPIObject() {}
func (*PolicyList) IsAnAPIObject()          {}
func (*PolicyBindingList) IsAnAPIObject()   {}
