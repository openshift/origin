package rulevalidation

import (
	"k8s.io/apiserver/pkg/authentication/user"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationinterfaces "github.com/openshift/origin/pkg/authorization/interfaces"
)

type AuthorizationRuleResolver interface {
	GetRoleBindings(namespace string) ([]authorizationinterfaces.RoleBinding, error)
	GetRole(roleBinding authorizationinterfaces.RoleBinding) (authorizationinterfaces.Role, error)
	// RulesFor returns the list of rules that apply to a given user in a given namespace and error.  If an error is returned, the slice of
	// PolicyRules may not be complete, but it contains all retrievable rules.  This is done because policy rules are purely additive and policy determinations
	// can be made on the basis of those rules that are found.
	RulesFor(info user.Info, namespace string) ([]authorizationapi.PolicyRule, error)
}
