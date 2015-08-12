package interfaces

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type Policy interface {
	Name() string
	Namespace() string

	Roles() map[string]Role
}

type PolicyBinding interface {
	Name() string
	Namespace() string

	PolicyRef() kapi.ObjectReference
	RoleBindings() map[string]RoleBinding
}

type Role interface {
	Name() string
	Namespace() string

	Rules() []authorizationapi.PolicyRule
}

type RoleBinding interface {
	Name() string
	Namespace() string

	RoleRef() kapi.ObjectReference
	Users() util.StringSet
	Groups() util.StringSet
}

func NewClusterPolicyAdapter(policy *authorizationapi.ClusterPolicy) Policy {
	return ClusterPolicyAdapter{policy: policy}
}
func NewLocalPolicyAdapter(policy *authorizationapi.Policy) Policy {
	return PolicyAdapter{policy: policy}
}

func NewClusterPolicyBindingAdapter(policyBinding *authorizationapi.ClusterPolicyBinding) PolicyBinding {
	return ClusterPolicyBindingAdapter{policyBinding: policyBinding}
}
func NewLocalPolicyBindingAdapter(policyBinding *authorizationapi.PolicyBinding) PolicyBinding {
	return PolicyBindingAdapter{policyBinding: policyBinding}
}
func NewClusterPolicyBindingAdapters(list *authorizationapi.ClusterPolicyBindingList) []PolicyBinding {
	ret := make([]PolicyBinding, 0, len(list.Items))
	for i := range list.Items {
		ret = append(ret, NewClusterPolicyBindingAdapter(&list.Items[i]))
	}
	return ret
}
func NewLocalPolicyBindingAdapters(list *authorizationapi.PolicyBindingList) []PolicyBinding {
	ret := make([]PolicyBinding, 0, len(list.Items))
	for i := range list.Items {
		ret = append(ret, NewLocalPolicyBindingAdapter(&list.Items[i]))
	}
	return ret
}

func NewClusterRoleBindingAdapter(roleBinding *authorizationapi.ClusterRoleBinding) RoleBinding {
	return ClusterRoleBindingAdapter{roleBinding: roleBinding}
}
func NewLocalRoleBindingAdapter(roleBinding *authorizationapi.RoleBinding) RoleBinding {
	return RoleBindingAdapter{roleBinding: roleBinding}
}

func NewClusterRoleAdapter(role *authorizationapi.ClusterRole) Role {
	return ClusterRoleAdapter{role: role}
}
func NewLocalRoleAdapter(role *authorizationapi.Role) Role {
	return RoleAdapter{role: role}
}

type PolicyAdapter struct {
	policy *authorizationapi.Policy

	adaptedRoles map[string]Role
}

func (a PolicyAdapter) Name() string {
	return a.policy.Name
}

func (a PolicyAdapter) Namespace() string {
	return a.policy.Namespace
}

func (a PolicyAdapter) Roles() map[string]Role {
	if a.adaptedRoles == nil {
		adaptedRoles := map[string]Role{}
		for key := range a.policy.Roles {
			adaptedRoles[key] = RoleAdapter{a.policy.Roles[key]}
		}
		a.adaptedRoles = adaptedRoles
	}
	return a.adaptedRoles
}

type RoleAdapter struct {
	role *authorizationapi.Role
}

func (a RoleAdapter) Name() string {
	return a.role.Name
}

func (a RoleAdapter) Namespace() string {
	return a.role.Namespace
}

func (a RoleAdapter) Rules() []authorizationapi.PolicyRule {
	return a.role.Rules
}

type ClusterPolicyAdapter struct {
	policy *authorizationapi.ClusterPolicy

	adaptedRoles map[string]Role
}

func (a ClusterPolicyAdapter) Name() string {
	return a.policy.Name
}

func (a ClusterPolicyAdapter) Namespace() string {
	return a.policy.Namespace
}

func (a ClusterPolicyAdapter) Roles() map[string]Role {
	if a.adaptedRoles == nil {
		adaptedRoles := map[string]Role{}
		for key := range a.policy.Roles {
			adaptedRoles[key] = ClusterRoleAdapter{a.policy.Roles[key]}
		}
		a.adaptedRoles = adaptedRoles
	}
	return a.adaptedRoles
}

type ClusterRoleAdapter struct {
	role *authorizationapi.ClusterRole
}

func (a ClusterRoleAdapter) Name() string {
	return a.role.Name
}

func (a ClusterRoleAdapter) Namespace() string {
	return a.role.Namespace
}

func (a ClusterRoleAdapter) Rules() []authorizationapi.PolicyRule {
	return a.role.Rules
}

type PolicyBindingAdapter struct {
	policyBinding *authorizationapi.PolicyBinding

	adaptedRoleBindings map[string]RoleBinding
}

func (a PolicyBindingAdapter) Name() string {
	return a.policyBinding.Name
}

func (a PolicyBindingAdapter) Namespace() string {
	return a.policyBinding.Namespace
}

func (a PolicyBindingAdapter) PolicyRef() kapi.ObjectReference {
	return a.policyBinding.PolicyRef
}

func (a PolicyBindingAdapter) RoleBindings() map[string]RoleBinding {
	if a.adaptedRoleBindings == nil {
		adaptedRoleBindings := map[string]RoleBinding{}
		for key := range a.policyBinding.RoleBindings {
			adaptedRoleBindings[key] = RoleBindingAdapter{a.policyBinding.RoleBindings[key]}
		}
		a.adaptedRoleBindings = adaptedRoleBindings
	}
	return a.adaptedRoleBindings
}

type RoleBindingAdapter struct {
	roleBinding *authorizationapi.RoleBinding
}

func (a RoleBindingAdapter) Name() string {
	return a.roleBinding.Name
}

func (a RoleBindingAdapter) Namespace() string {
	return a.roleBinding.Namespace
}

func (a RoleBindingAdapter) RoleRef() kapi.ObjectReference {
	return a.roleBinding.RoleRef
}

func (a RoleBindingAdapter) Users() util.StringSet {
	return a.roleBinding.Users
}
func (a RoleBindingAdapter) Groups() util.StringSet {
	return a.roleBinding.Groups
}

type ClusterPolicyBindingAdapter struct {
	policyBinding *authorizationapi.ClusterPolicyBinding

	adaptedRoleBindings map[string]RoleBinding
}

func (a ClusterPolicyBindingAdapter) Name() string {
	return a.policyBinding.Name
}

func (a ClusterPolicyBindingAdapter) Namespace() string {
	return a.policyBinding.Namespace
}

func (a ClusterPolicyBindingAdapter) PolicyRef() kapi.ObjectReference {
	return a.policyBinding.PolicyRef
}

func (a ClusterPolicyBindingAdapter) RoleBindings() map[string]RoleBinding {
	if a.adaptedRoleBindings == nil {
		adaptedRoleBindings := map[string]RoleBinding{}
		for key := range a.policyBinding.RoleBindings {
			adaptedRoleBindings[key] = ClusterRoleBindingAdapter{a.policyBinding.RoleBindings[key]}
		}
		a.adaptedRoleBindings = adaptedRoleBindings
	}
	return a.adaptedRoleBindings
}

type ClusterRoleBindingAdapter struct {
	roleBinding *authorizationapi.ClusterRoleBinding
}

func (a ClusterRoleBindingAdapter) Name() string {
	return a.roleBinding.Name
}

func (a ClusterRoleBindingAdapter) Namespace() string {
	return a.roleBinding.Namespace
}

func (a ClusterRoleBindingAdapter) RoleRef() kapi.ObjectReference {
	return a.roleBinding.RoleRef
}

func (a ClusterRoleBindingAdapter) Users() util.StringSet {
	return a.roleBinding.Users
}
func (a ClusterRoleBindingAdapter) Groups() util.StringSet {
	return a.roleBinding.Groups
}
