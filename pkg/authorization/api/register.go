package api

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

const GroupName = ""

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: GroupName, Version: ""}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) unversioned.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns back a Group qualified GroupResource
func Resource(resource string) unversioned.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func AddToScheme(scheme *runtime.Scheme) {
	// Add the API to Scheme.
	addKnownTypes(scheme)
}

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) {
	scheme.AddKnownTypes(SchemeGroupVersion,
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

func (obj *ClusterRoleList) GetObjectKind() unversioned.ObjectKind          { return &obj.TypeMeta }
func (obj *ClusterRoleBindingList) GetObjectKind() unversioned.ObjectKind   { return &obj.TypeMeta }
func (obj *ClusterPolicyBindingList) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
func (obj *ClusterPolicyList) GetObjectKind() unversioned.ObjectKind        { return &obj.TypeMeta }
func (obj *ClusterPolicyBinding) GetObjectKind() unversioned.ObjectKind     { return &obj.TypeMeta }
func (obj *ClusterPolicy) GetObjectKind() unversioned.ObjectKind            { return &obj.TypeMeta }
func (obj *ClusterRoleBinding) GetObjectKind() unversioned.ObjectKind       { return &obj.TypeMeta }
func (obj *ClusterRole) GetObjectKind() unversioned.ObjectKind              { return &obj.TypeMeta }

func (obj *IsPersonalSubjectAccessReview) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
func (obj *SubjectAccessReviewResponse) GetObjectKind() unversioned.ObjectKind   { return &obj.TypeMeta }
func (obj *ResourceAccessReviewResponse) GetObjectKind() unversioned.ObjectKind  { return &obj.TypeMeta }
func (obj *LocalSubjectAccessReview) GetObjectKind() unversioned.ObjectKind      { return &obj.TypeMeta }
func (obj *LocalResourceAccessReview) GetObjectKind() unversioned.ObjectKind     { return &obj.TypeMeta }
func (obj *SubjectAccessReview) GetObjectKind() unversioned.ObjectKind           { return &obj.TypeMeta }
func (obj *ResourceAccessReview) GetObjectKind() unversioned.ObjectKind          { return &obj.TypeMeta }

func (obj *RoleList) GetObjectKind() unversioned.ObjectKind          { return &obj.TypeMeta }
func (obj *RoleBindingList) GetObjectKind() unversioned.ObjectKind   { return &obj.TypeMeta }
func (obj *PolicyBindingList) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
func (obj *PolicyList) GetObjectKind() unversioned.ObjectKind        { return &obj.TypeMeta }
func (obj *PolicyBinding) GetObjectKind() unversioned.ObjectKind     { return &obj.TypeMeta }
func (obj *Policy) GetObjectKind() unversioned.ObjectKind            { return &obj.TypeMeta }
func (obj *RoleBinding) GetObjectKind() unversioned.ObjectKind       { return &obj.TypeMeta }
func (obj *Role) GetObjectKind() unversioned.ObjectKind              { return &obj.TypeMeta }
