package rulevalidation

import (
	"errors"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierror "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationinterfaces "github.com/openshift/origin/pkg/authorization/interfaces"
)

type DefaultRuleResolver struct {
	policyGetter  PolicyGetter
	bindingLister BindingLister

	clusterPolicyGetter  ClusterPolicyGetter
	clusterBindingLister ClusterBindingLister
}

func NewDefaultRuleResolver(policyGetter PolicyGetter, bindingLister BindingLister, clusterPolicyGetter ClusterPolicyGetter, clusterBindingLister ClusterBindingLister) *DefaultRuleResolver {
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

type PolicyGetter interface {
	GetPolicy(ctx kapi.Context, id string) (*authorizationapi.Policy, error)
}

type BindingLister interface {
	ListPolicyBindings(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.PolicyBindingList, error)
}

type ClusterPolicyGetter interface {
	GetClusterPolicy(ctx kapi.Context, id string) (*authorizationapi.ClusterPolicy, error)
}

type ClusterBindingLister interface {
	ListClusterPolicyBindings(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.ClusterPolicyBindingList, error)
}

func (a *DefaultRuleResolver) GetRoleBindings(ctx kapi.Context) ([]authorizationinterfaces.RoleBinding, error) {
	namespace := kapi.NamespaceValue(ctx)

	if len(namespace) == 0 {
		policyBindingList, err := a.clusterBindingLister.ListClusterPolicyBindings(ctx, labels.Everything(), fields.Everything())
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

	policyBindingList, err := a.bindingLister.ListPolicyBindings(ctx, labels.Everything(), fields.Everything())
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
	ctx := kapi.WithNamespace(kapi.NewContext(), namespace)

	if len(namespace) == 0 {

		policy, err := a.clusterPolicyGetter.GetClusterPolicy(ctx, authorizationapi.PolicyName)
		if kapierror.IsNotFound(err) {
			return nil, kapierror.NewNotFound("role", name)
		}
		if err != nil {
			return nil, err
		}

		role, exists := policy.Roles[name]
		if !exists {
			return nil, kapierror.NewNotFound("role", name)
		}

		return authorizationinterfaces.NewClusterRoleAdapter(role), nil
	}

	policy, err := a.policyGetter.GetPolicy(ctx, authorizationapi.PolicyName)
	if kapierror.IsNotFound(err) {
		return nil, kapierror.NewNotFound("role", name)
	}
	if err != nil {
		return nil, err
	}

	role, exists := policy.Roles[name]
	if !exists {
		return nil, kapierror.NewNotFound("role", name)
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
		if !appliesToUser(roleBinding.Users(), roleBinding.Groups(), user) {
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
