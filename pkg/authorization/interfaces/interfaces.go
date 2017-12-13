package interfaces

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

type Role interface {
	Name() string
	Namespace() string

	Rules() []authorizationapi.PolicyRule
}

type RoleBinding interface {
	Name() string
	Namespace() string

	RoleRef() kapi.ObjectReference
	Users() sets.String
	Groups() sets.String

	// AppliesToUser returns true if the provided user matches this role binding
	AppliesToUser(user.Info) bool
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

func (a RoleBindingAdapter) Users() sets.String {
	users, _ := authorizationapi.StringSubjectsFor(a.roleBinding.Namespace, a.roleBinding.Subjects)

	return sets.NewString(users...)
}

func (a RoleBindingAdapter) Groups() sets.String {
	_, groups := authorizationapi.StringSubjectsFor(a.roleBinding.Namespace, a.roleBinding.Subjects)

	return sets.NewString(groups...)
}

// AppliesToUser returns true if this binding applies to the provided user.
func (a RoleBindingAdapter) AppliesToUser(user user.Info) bool {
	if subjectsContainUser(a.roleBinding.Subjects, a.roleBinding.Namespace, user.GetName()) {
		return true
	}
	if subjectsContainAnyGroup(a.roleBinding.Subjects, user.GetGroups()) {
		return true
	}
	return false
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

func (a ClusterRoleBindingAdapter) Users() sets.String {
	users, _ := authorizationapi.StringSubjectsFor(a.roleBinding.Namespace, a.roleBinding.Subjects)

	return sets.NewString(users...)
}
func (a ClusterRoleBindingAdapter) Groups() sets.String {
	_, groups := authorizationapi.StringSubjectsFor(a.roleBinding.Namespace, a.roleBinding.Subjects)

	return sets.NewString(groups...)
}

// AppliesToUser returns true if this binding applies to the provided user.
func (a ClusterRoleBindingAdapter) AppliesToUser(user user.Info) bool {
	if subjectsContainUser(a.roleBinding.Subjects, a.roleBinding.Namespace, user.GetName()) {
		return true
	}
	if subjectsContainAnyGroup(a.roleBinding.Subjects, user.GetGroups()) {
		return true
	}
	return false
}

// subjectsContainUser returns true if the provided subjects contain the named user. currentNamespace
// is used to identify service accounts that are defined in a relative fashion.
func subjectsContainUser(subjects []kapi.ObjectReference, currentNamespace string, user string) bool {
	if !strings.HasPrefix(user, serviceaccount.ServiceAccountUsernamePrefix) {
		for _, subject := range subjects {
			switch subject.Kind {
			case authorizationapi.UserKind, authorizationapi.SystemUserKind:
				if user == subject.Name {
					return true
				}
			}
		}
		return false
	}

	for _, subject := range subjects {
		switch subject.Kind {
		case authorizationapi.ServiceAccountKind:
			namespace := currentNamespace
			if len(subject.Namespace) > 0 {
				namespace = subject.Namespace
			}
			if len(namespace) == 0 {
				continue
			}
			if user == serviceaccount.MakeUsername(namespace, subject.Name) {
				return true
			}

		case authorizationapi.UserKind, authorizationapi.SystemUserKind:
			if user == subject.Name {
				return true
			}
		}
	}
	return false
}

// subjectsContainAnyGroup returns true if the provided subjects any of the named groups.
func subjectsContainAnyGroup(subjects []kapi.ObjectReference, groups []string) bool {
	for _, subject := range subjects {
		switch subject.Kind {
		case authorizationapi.GroupKind, authorizationapi.SystemGroupKind:
			for _, group := range groups {
				if group == subject.Name {
					return true
				}
			}
		}
	}
	return false
}
