package v1beta3

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta3",
		&Role{},
		&RoleBinding{},
		&Policy{},
		&PolicyBinding{},
		&PolicyList{},
		&PolicyBindingList{},
		&RoleBindingList{},
		&RoleList{},

		&ResourceAccessReview{},
		&SubjectAccessReview{},
		&LocalResourceAccessReview{},
		&LocalSubjectAccessReview{},
		&ResourceAccessReviewResponse{},
		&SubjectAccessReviewResponse{},
		&IsPersonalSubjectAccessReview{},

		&ClusterRole{},
		&ClusterRoleBinding{},
		&ClusterPolicy{},
		&ClusterPolicyBinding{},
		&ClusterPolicyList{},
		&ClusterPolicyBindingList{},
		&ClusterRoleBindingList{},
		&ClusterRoleList{},
	)
}

func (*ClusterRole) IsAnAPIObject()              {}
func (*ClusterPolicy) IsAnAPIObject()            {}
func (*ClusterPolicyBinding) IsAnAPIObject()     {}
func (*ClusterRoleBinding) IsAnAPIObject()       {}
func (*ClusterPolicyList) IsAnAPIObject()        {}
func (*ClusterPolicyBindingList) IsAnAPIObject() {}
func (*ClusterRoleBindingList) IsAnAPIObject()   {}
func (*ClusterRoleList) IsAnAPIObject()          {}

func (*Role) IsAnAPIObject()              {}
func (*Policy) IsAnAPIObject()            {}
func (*PolicyBinding) IsAnAPIObject()     {}
func (*RoleBinding) IsAnAPIObject()       {}
func (*PolicyList) IsAnAPIObject()        {}
func (*PolicyBindingList) IsAnAPIObject() {}
func (*RoleBindingList) IsAnAPIObject()   {}
func (*RoleList) IsAnAPIObject()          {}

func (*ResourceAccessReview) IsAnAPIObject()          {}
func (*SubjectAccessReview) IsAnAPIObject()           {}
func (*LocalResourceAccessReview) IsAnAPIObject()     {}
func (*LocalSubjectAccessReview) IsAnAPIObject()      {}
func (*ResourceAccessReviewResponse) IsAnAPIObject()  {}
func (*SubjectAccessReviewResponse) IsAnAPIObject()   {}
func (*IsPersonalSubjectAccessReview) IsAnAPIObject() {}
