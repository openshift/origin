package authorization

import (
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

func ToRoleList(in *ClusterRoleList) *RoleList {
	ret := &RoleList{}
	for _, curr := range in.Items {
		ret.Items = append(ret.Items, *ToRole(&curr))
	}

	return ret
}

func ToRole(in *ClusterRole) *Role {
	if in == nil {
		return nil
	}

	ret := &Role{}
	ret.ObjectMeta = in.ObjectMeta
	ret.Rules = in.Rules

	return ret
}

func ToRoleBindingList(in *ClusterRoleBindingList) *RoleBindingList {
	ret := &RoleBindingList{}
	for _, curr := range in.Items {
		ret.Items = append(ret.Items, *ToRoleBinding(&curr))
	}

	return ret
}

func ToRoleBinding(in *ClusterRoleBinding) *RoleBinding {
	if in == nil {
		return nil
	}

	ret := &RoleBinding{}
	ret.ObjectMeta = in.ObjectMeta
	ret.Subjects = in.Subjects
	ret.RoleRef = ToRoleRef(in.RoleRef)
	return ret
}

func ToRoleRef(in kapi.ObjectReference) kapi.ObjectReference {
	ret := kapi.ObjectReference{}

	ret.Name = in.Name
	return ret
}
