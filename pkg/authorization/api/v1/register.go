package v1

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: "v1"}

// Codec encodes internal objects to the v1 scheme
var Codec = runtime.CodecFor(api.Scheme, SchemeGroupVersion.String())

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
