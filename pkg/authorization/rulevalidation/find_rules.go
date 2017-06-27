package rulevalidation

import (
	kapierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/authentication/user"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationlister "github.com/openshift/origin/pkg/authorization/generated/listers/authorization/internalversion"
	authorizationinterfaces "github.com/openshift/origin/pkg/authorization/interfaces"
)

type DefaultRuleResolver struct {
	policyGetter  authorizationlister.PolicyLister
	bindingLister authorizationlister.PolicyBindingLister

	clusterPolicyGetter  authorizationlister.ClusterPolicyLister
	clusterBindingLister authorizationlister.ClusterPolicyBindingLister
}

func NewDefaultRuleResolver(policyGetter authorizationlister.PolicyLister, bindingLister authorizationlister.PolicyBindingLister, clusterPolicyGetter authorizationlister.ClusterPolicyLister, clusterBindingLister authorizationlister.ClusterPolicyBindingLister) *DefaultRuleResolver {
	return &DefaultRuleResolver{policyGetter, bindingLister, clusterPolicyGetter, clusterBindingLister}
}

type AuthorizationRuleResolver interface {
	GetRoleBindings(namespace string) ([]authorizationinterfaces.RoleBinding, error)
	GetRole(roleBinding authorizationinterfaces.RoleBinding) (authorizationinterfaces.Role, error)
	// RulesFor returns the list of rules that apply to a given user in a given namespace and error.  If an error is returned, the slice of
	// PolicyRules may not be complete, but it contains all retrievable rules.  This is done because policy rules are purely additive and policy determinations
	// can be made on the basis of those rules that are found.
	RulesFor(info user.Info, namespace string) ([]authorizationapi.PolicyRule, error)
}

func (a *DefaultRuleResolver) GetRoleBindings(namespace string) ([]authorizationinterfaces.RoleBinding, error) {
	clusterBindings, clusterErr := a.clusterBindingLister.List(labels.Everything())

	var namespaceBindings []*authorizationapi.PolicyBinding
	var namespaceErr error
	if a.bindingLister != nil && len(namespace) > 0 {
		namespaceBindings, namespaceErr = a.bindingLister.PolicyBindings(namespace).List(labels.Everything())
	}

	// return all loaded bindings
	expect := 0
	if clusterBindings != nil {
		expect += len(clusterBindings)
	}
	if namespaceBindings != nil {
		expect += len(namespaceBindings)
	}
	bindings := make([]authorizationinterfaces.RoleBinding, 0, expect)
	if clusterBindings != nil {
		for _, policyBinding := range clusterBindings {
			for _, value := range policyBinding.RoleBindings {
				bindings = append(bindings, authorizationinterfaces.NewClusterRoleBindingAdapter(value))
			}
		}
	}
	if namespaceBindings != nil {
		for _, policyBinding := range namespaceBindings {
			for _, value := range policyBinding.RoleBindings {
				bindings = append(bindings, authorizationinterfaces.NewLocalRoleBindingAdapter(value))
			}
		}
	}

	// return all errors
	var errs []error
	if clusterErr != nil {
		errs = append(errs, clusterErr)
	}
	if namespaceErr != nil {
		errs = append(errs, namespaceErr)
	}

	return bindings, kerrors.NewAggregate(errs)
}

func (a *DefaultRuleResolver) GetRole(roleBinding authorizationinterfaces.RoleBinding) (authorizationinterfaces.Role, error) {
	namespace := roleBinding.RoleRef().Namespace
	name := roleBinding.RoleRef().Name

	if len(namespace) == 0 {
		policy, err := a.clusterPolicyGetter.Get(authorizationapi.PolicyName)
		if kapierror.IsNotFound(err) {
			return nil, kapierror.NewNotFound(authorizationapi.Resource("role"), name)
		}
		if err != nil {
			return nil, err
		}

		role, exists := policy.Roles[name]
		if !exists {
			return nil, kapierror.NewNotFound(authorizationapi.Resource("role"), name)
		}

		return authorizationinterfaces.NewClusterRoleAdapter(role), nil
	}

	if a.policyGetter == nil {
		return nil, kapierror.NewNotFound(authorizationapi.Resource("role"), name)
	}

	policy, err := a.policyGetter.Policies(namespace).Get(authorizationapi.PolicyName)
	if kapierror.IsNotFound(err) {
		return nil, kapierror.NewNotFound(authorizationapi.Resource("role"), name)
	}
	if err != nil {
		return nil, err
	}

	role, exists := policy.Roles[name]
	if !exists {
		return nil, kapierror.NewNotFound(authorizationapi.Resource("role"), name)
	}

	return authorizationinterfaces.NewLocalRoleAdapter(role), nil

}

// RulesFor returns the list of rules that apply to a given user in a given namespace and error.  If an error is returned, the slice of
// PolicyRules may not be complete, but it contains all retrievable rules.  This is done because policy rules are purely additive and policy determinations
// can be made on the basis of those rules that are found.
func (a *DefaultRuleResolver) RulesFor(user user.Info, namespace string) ([]authorizationapi.PolicyRule, error) {
	var errs []error

	roleBindings, err := a.GetRoleBindings(namespace)
	if err != nil {
		errs = append(errs, err)
	}

	rules := make([]authorizationapi.PolicyRule, 0, len(roleBindings))
	for _, roleBinding := range roleBindings {
		if !roleBinding.AppliesToUser(user) {
			continue
		}

		role, err := a.GetRole(roleBinding)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, curr := range role.Rules() {
			rules = append(rules, curr)
		}
	}

	return rules, kerrors.NewAggregate(errs)
}
