package v1beta3

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta3",
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

		&IsPersonalSubjectAccessReview{},

		&ClusterRole{},
		&ClusterRoleBinding{},
		&ClusterPolicy{},
		&ClusterPolicyBinding{},
		&ClusterPolicyList{},
		&ClusterPolicyBindingList{},
		&ClusterRoleBindingList{},
		&ClusterRoleList{},
		&IsVerbAllowedOnBaseResource{},
	)
}

func (*IsVerbAllowedOnBaseResource) IsAnAPIObject() {}

func (*ClusterRole) IsAnAPIObject()              {}
func (*ClusterPolicy) IsAnAPIObject()            {}
func (*ClusterPolicyBinding) IsAnAPIObject()     {}
func (*ClusterRoleBinding) IsAnAPIObject()       {}
func (*ClusterPolicyList) IsAnAPIObject()        {}
func (*ClusterPolicyBindingList) IsAnAPIObject() {}
func (*ClusterRoleBindingList) IsAnAPIObject()   {}
func (*ClusterRoleList) IsAnAPIObject()          {}

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

func (*IsPersonalSubjectAccessReview) IsAnAPIObject() {}
