package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

type TypeConverter struct {
	MasterNamespace string
}

// policies

func (t TypeConverter) ToPolicyList(in *ClusterPolicyList) *PolicyList {
	ret := &PolicyList{}
	for _, curr := range in.Items {
		ret.Items = append(ret.Items, *t.ToPolicy(&curr))
	}

	return ret
}

func (t TypeConverter) ToPolicy(in *ClusterPolicy) *Policy {
	if in == nil {
		return nil
	}

	ret := &Policy{}
	ret.ObjectMeta = in.ObjectMeta
	ret.Namespace = t.MasterNamespace
	ret.LastModified = in.LastModified
	ret.Roles = t.ToRoleMap(in.Roles)

	return ret
}

func (t TypeConverter) ToRoleMap(in map[string]ClusterRole) map[string]Role {
	ret := map[string]Role{}
	for key, role := range in {
		ret[key] = *t.ToRole(&role)
	}

	return ret
}

func (t TypeConverter) ToRoleList(in *ClusterRoleList) *RoleList {
	ret := &RoleList{}
	for _, curr := range in.Items {
		ret.Items = append(ret.Items, *t.ToRole(&curr))
	}

	return ret
}

func (t TypeConverter) ToRole(in *ClusterRole) *Role {
	if in == nil {
		return nil
	}

	ret := &Role{}
	ret.ObjectMeta = in.ObjectMeta
	ret.Namespace = t.MasterNamespace
	ret.Rules = in.Rules

	return ret
}

func (t TypeConverter) ToClusterPolicyList(in *PolicyList) *ClusterPolicyList {
	ret := &ClusterPolicyList{}
	for _, curr := range in.Items {
		ret.Items = append(ret.Items, *t.ToClusterPolicy(&curr))
	}

	return ret
}

func (t TypeConverter) ToClusterPolicy(in *Policy) *ClusterPolicy {
	if in == nil {
		return nil
	}

	ret := &ClusterPolicy{}
	ret.ObjectMeta = in.ObjectMeta
	ret.Namespace = ""
	ret.LastModified = in.LastModified
	ret.Roles = t.ToClusterRoleMap(in.Roles)

	return ret
}

func (t TypeConverter) ToClusterRoleMap(in map[string]Role) map[string]ClusterRole {
	ret := map[string]ClusterRole{}
	for key, role := range in {
		ret[key] = *t.ToClusterRole(&role)
	}

	return ret
}

func (t TypeConverter) ToClusterRoleList(in *RoleList) *ClusterRoleList {
	ret := &ClusterRoleList{}
	for _, curr := range in.Items {
		ret.Items = append(ret.Items, *t.ToClusterRole(&curr))
	}

	return ret
}

func (t TypeConverter) ToClusterRole(in *Role) *ClusterRole {
	if in == nil {
		return nil
	}

	ret := &ClusterRole{}
	ret.ObjectMeta = in.ObjectMeta
	ret.Namespace = ""
	ret.Rules = in.Rules

	return ret
}

// policy bindings

func (t TypeConverter) ToPolicyBindingList(in *ClusterPolicyBindingList) *PolicyBindingList {
	ret := &PolicyBindingList{}
	for _, curr := range in.Items {
		ret.Items = append(ret.Items, *t.ToPolicyBinding(&curr))
	}

	return ret
}

func (t TypeConverter) ToPolicyBinding(in *ClusterPolicyBinding) *PolicyBinding {
	if in == nil {
		return nil
	}

	ret := &PolicyBinding{}
	ret.ObjectMeta = in.ObjectMeta
	ret.Namespace = t.MasterNamespace
	ret.LastModified = in.LastModified
	ret.PolicyRef = t.ToPolicyRef(in.PolicyRef)
	ret.RoleBindings = t.ToRoleBindingMap(in.RoleBindings)

	return ret
}

func (t TypeConverter) ToPolicyRef(in kapi.ObjectReference) kapi.ObjectReference {
	ret := kapi.ObjectReference{}
	ret.Namespace = t.MasterNamespace
	ret.Name = in.Name
	return ret
}

func (t TypeConverter) ToRoleBindingMap(in map[string]ClusterRoleBinding) map[string]RoleBinding {
	ret := map[string]RoleBinding{}
	for key, RoleBinding := range in {
		ret[key] = *t.ToRoleBinding(&RoleBinding)
	}

	return ret
}

func (t TypeConverter) ToRoleBindingList(in *ClusterRoleBindingList) *RoleBindingList {
	ret := &RoleBindingList{}
	for _, curr := range in.Items {
		ret.Items = append(ret.Items, *t.ToRoleBinding(&curr))
	}

	return ret
}

func (t TypeConverter) ToRoleBinding(in *ClusterRoleBinding) *RoleBinding {
	if in == nil {
		return nil
	}

	ret := &RoleBinding{}
	ret.ObjectMeta = in.ObjectMeta
	ret.Namespace = t.MasterNamespace
	ret.Users = in.Users
	ret.Groups = in.Groups
	ret.RoleRef = t.ToRoleRef(in.RoleRef)
	return ret
}

func (t TypeConverter) ToRoleRef(in kapi.ObjectReference) kapi.ObjectReference {
	ret := kapi.ObjectReference{}
	ret.Namespace = t.MasterNamespace
	ret.Name = in.Name
	return ret
}

func (t TypeConverter) ToClusterPolicyBindingList(in *PolicyBindingList) *ClusterPolicyBindingList {
	ret := &ClusterPolicyBindingList{}
	for _, curr := range in.Items {
		ret.Items = append(ret.Items, *t.ToClusterPolicyBinding(&curr))
	}

	return ret
}

func (t TypeConverter) ToClusterPolicyBinding(in *PolicyBinding) *ClusterPolicyBinding {
	if in == nil {
		return nil
	}

	ret := &ClusterPolicyBinding{}
	ret.ObjectMeta = in.ObjectMeta
	ret.Namespace = ""
	ret.LastModified = in.LastModified
	ret.PolicyRef = t.ToClusterPolicyRef(in.PolicyRef)
	ret.RoleBindings = t.ToClusterRoleBindingMap(in.RoleBindings)

	return ret
}

func (t TypeConverter) ToClusterPolicyRef(in kapi.ObjectReference) kapi.ObjectReference {
	ret := kapi.ObjectReference{}
	ret.Namespace = ""
	ret.Name = in.Name
	return ret
}

func (t TypeConverter) ToClusterRoleBindingMap(in map[string]RoleBinding) map[string]ClusterRoleBinding {
	ret := map[string]ClusterRoleBinding{}
	for key, RoleBinding := range in {
		ret[key] = *t.ToClusterRoleBinding(&RoleBinding)
	}

	return ret
}

func (t TypeConverter) ToClusterRoleBindingList(in *RoleBindingList) *ClusterRoleBindingList {
	ret := &ClusterRoleBindingList{}
	for _, curr := range in.Items {
		ret.Items = append(ret.Items, *t.ToClusterRoleBinding(&curr))
	}

	return ret
}

func (t TypeConverter) ToClusterRoleBinding(in *RoleBinding) *ClusterRoleBinding {
	if in == nil {
		return nil
	}

	ret := &ClusterRoleBinding{}
	ret.ObjectMeta = in.ObjectMeta
	ret.Namespace = ""
	ret.Users = in.Users
	ret.Groups = in.Groups
	ret.RoleRef = t.ToClusterRoleRef(in.RoleRef)

	return ret
}

func (t TypeConverter) ToClusterRoleRef(in kapi.ObjectReference) kapi.ObjectReference {
	ret := kapi.ObjectReference{}
	ret.Namespace = ""
	ret.Name = in.Name
	return ret
}
