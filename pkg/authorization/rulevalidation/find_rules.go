package rulevalidation

import (
	"errors"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierror "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/auth/user"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationinterfaces "github.com/openshift/origin/pkg/authorization/interfaces"
	"github.com/openshift/origin/pkg/client"
)

type DefaultRuleResolver struct {
	policyGetter  client.PoliciesListerNamespacer
	bindingLister client.PolicyBindingsListerNamespacer

	clusterPolicyGetter  client.ClusterPolicyLister
	clusterBindingLister client.ClusterPolicyBindingLister
}

func NewDefaultRuleResolver(policyGetter client.PoliciesListerNamespacer, bindingLister client.PolicyBindingsListerNamespacer, clusterPolicyGetter client.ClusterPolicyLister, clusterBindingLister client.ClusterPolicyBindingLister) *DefaultRuleResolver {
	return &DefaultRuleResolver{policyGetter, bindingLister, clusterPolicyGetter, clusterBindingLister}
}

type AuthorizationRuleResolver interface {
	GetRoleBindings(ctx kapi.Context) ([]authorizationinterfaces.RoleBinding, error)
	GetRole(roleBinding authorizationinterfaces.RoleBinding) (authorizationinterfaces.Role, error)
	// GetEffectivePolicyRules returns the list of rules that apply to a given user in a given namespace and error.  If an error is returned, the slice of
	// PolicyRules may not be complete, but it contains all retrievable rules.  This is done because policy rules are purely additive and policy determinations
	// can be made on the basis of those rules that are found.
	GetEffectivePolicyRules(ctx kapi.Context) ([]authorizationapi.PolicyRule, error)
}

func (a *DefaultRuleResolver) GetRoleBindings(ctx kapi.Context) ([]authorizationinterfaces.RoleBinding, error) {
	namespace := kapi.NamespaceValue(ctx)

	if len(namespace) == 0 {
		policyBindingList, err := a.clusterBindingLister.List(kapi.ListOptions{})
		if err != nil {
			return nil, err
		}

		ret := make([]authorizationinterfaces.RoleBinding, 0, len(policyBindingList.Items))
		for _, policyBinding := range policyBindingList.Items {
			for _, value := range policyBinding.RoleBindings {
				ret = append(ret, authorizationinterfaces.NewClusterRoleBindingAdapter(value))
			}
		}
		return ret, nil
	}

	if a.bindingLister == nil {
		return nil, nil
	}

	policyBindingList, err := a.bindingLister.PolicyBindings(namespace).List(kapi.ListOptions{})
	if err != nil {
		return nil, err
	}

	ret := make([]authorizationinterfaces.RoleBinding, 0, len(policyBindingList.Items))
	for _, policyBinding := range policyBindingList.Items {
		for _, value := range policyBinding.RoleBindings {
			ret = append(ret, authorizationinterfaces.NewLocalRoleBindingAdapter(value))
		}
	}
	return ret, nil
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

// GetEffectivePolicyRules returns the list of rules that apply to a given user in a given namespace and error.  If an error is returned, the slice of
// PolicyRules may not be complete, but it contains all retrievable rules.  This is done because policy rules are purely additive and policy determinations
// can be made on the basis of those rules that are found.
func (a *DefaultRuleResolver) GetEffectivePolicyRules(ctx kapi.Context) ([]authorizationapi.PolicyRule, error) {
	roleBindings, err := a.GetRoleBindings(ctx)
	if err != nil {
		return nil, err
	}
	user, exists := kapi.UserFrom(ctx)
	if !exists {
		return nil, errors.New("user missing from context")
	}

	errs := []error{}
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
func appliesToUser(ruleUsers, ruleGroups sets.String, user user.Info) bool {
	if ruleUsers.Has(user.GetName()) {
		return true
	}

	for _, currGroup := range user.GetGroups() {
		if ruleGroups.Has(currGroup) {
			return true
		}
	}

	return false
}
