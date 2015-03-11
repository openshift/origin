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
		&ResourceAccessReview{},
		&SubjectAccessReview{},
		&ResourceAccessReviewResponse{},
		&SubjectAccessReviewResponse{},
		&PolicyList{},
		&PolicyBindingList{},
		&RoleBindingList{},
		&RoleList{},
	)
}

func (*Role) IsAnAPIObject()                         {}
func (*Policy) IsAnAPIObject()                       {}
func (*PolicyBinding) IsAnAPIObject()                {}
func (*RoleBinding) IsAnAPIObject()                  {}
func (*ResourceAccessReview) IsAnAPIObject()         {}
func (*SubjectAccessReview) IsAnAPIObject()          {}
func (*ResourceAccessReviewResponse) IsAnAPIObject() {}
func (*SubjectAccessReviewResponse) IsAnAPIObject()  {}
func (*PolicyList) IsAnAPIObject()                   {}
func (*PolicyBindingList) IsAnAPIObject()            {}
func (*RoleBindingList) IsAnAPIObject()              {}
func (*RoleList) IsAnAPIObject()                     {}
