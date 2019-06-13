package authorization

import (
	corev1 "k8s.io/api/core/v1"

	authorizationv1 "github.com/openshift/api/authorization/v1"
)

func ToRoleList(in *authorizationv1.ClusterRoleList) *authorizationv1.RoleList {
	ret := &authorizationv1.RoleList{}
	for _, curr := range in.Items {
		ret.Items = append(ret.Items, *ToRole(&curr))
	}

	return ret
}

func ToRole(in *authorizationv1.ClusterRole) *authorizationv1.Role {
	if in == nil {
		return nil
	}

	ret := &authorizationv1.Role{}
	ret.ObjectMeta = in.ObjectMeta
	ret.Rules = in.Rules

	return ret
}

func ToRoleBindingList(in *authorizationv1.ClusterRoleBindingList) *authorizationv1.RoleBindingList {
	ret := &authorizationv1.RoleBindingList{}
	for _, curr := range in.Items {
		ret.Items = append(ret.Items, *ToRoleBinding(&curr))
	}

	return ret
}

func ToRoleBinding(in *authorizationv1.ClusterRoleBinding) *authorizationv1.RoleBinding {
	if in == nil {
		return nil
	}

	ret := &authorizationv1.RoleBinding{}
	ret.ObjectMeta = in.ObjectMeta
	ret.Subjects = in.Subjects
	ret.RoleRef = ToRoleRef(in.RoleRef)
	return ret
}

func ToRoleRef(in corev1.ObjectReference) corev1.ObjectReference {
	ret := corev1.ObjectReference{}

	ret.Name = in.Name
	return ret
}
