package api

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: ""}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) unversioned.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns back a Group qualified GroupResource
func Resource(resource string) unversioned.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func init() {
	api.Scheme.AddKnownTypes(SchemeGroupVersion,
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
