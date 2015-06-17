package rulevalidation

import (
	"errors"
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierror "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

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

func (a *DefaultRuleResolver) getPolicy(ctx kapi.Context) (authorizationinterfaces.Policy, error) {
	namespace, _ := kapi.NamespaceFrom(ctx)

	switch {
	case len(namespace) == 0:
		t, err := a.clusterPolicyGetter.GetClusterPolicy(ctx, authorizationapi.PolicyName)
		if err != nil {
			return nil, err
		}
		return authorizationinterfaces.NewClusterPolicyAdapter(t), nil
	default:
		t, err := a.policyGetter.GetPolicy(ctx, authorizationapi.PolicyName)
		if err != nil {
			return nil, err
		}
		return authorizationinterfaces.NewLocalPolicyAdapter(t), nil
	}
}

func (a *DefaultRuleResolver) getPolicyBindings(ctx kapi.Context) ([]authorizationinterfaces.PolicyBinding, error) {
	namespace, _ := kapi.NamespaceFrom(ctx)

	switch {
	case len(namespace) == 0:
		t, err := a.clusterBindingLister.ListClusterPolicyBindings(ctx, labels.Everything(), fields.Everything())
		if err != nil {
			return nil, err
		}
		return authorizationinterfaces.NewClusterPolicyBindingAdapters(t), nil
	default:
		t, err := a.bindingLister.ListPolicyBindings(ctx, labels.Everything(), fields.Everything())
		if err != nil {
			return nil, err
		}
		return authorizationinterfaces.NewLocalPolicyBindingAdapters(t), nil
	}
}

func (a *DefaultRuleResolver) GetRoleBindings(ctx kapi.Context) ([]authorizationinterfaces.RoleBinding, error) {
	policyBindings, err := a.getPolicyBindings(ctx)
	if err != nil {
		return nil, err
	}

	ret := make([]authorizationinterfaces.RoleBinding, 0, len(policyBindings))
	for _, policyBinding := range policyBindings {
		for _, value := range policyBinding.RoleBindings() {
			ret = append(ret, value)
		}
	}

	return ret, nil
}

func (a *DefaultRuleResolver) GetRole(roleBinding authorizationinterfaces.RoleBinding) (authorizationinterfaces.Role, error) {
	namespace := roleBinding.RoleRef().Namespace
	name := roleBinding.RoleRef().Name

	ctx := kapi.WithNamespace(kapi.NewContext(), namespace)
	policy, err := a.getPolicy(ctx)
	if kapierror.IsNotFound(err) {
		return nil, kapierror.NewNotFound("role", roleBinding.RoleRef().Name)
	}
	if err != nil {
		return nil, err
	}

	role, exists := policy.Roles()[name]
	if !exists {
		return nil, fmt.Errorf("role %#v not found", roleBinding.RoleRef())
	}

	return role, nil
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
func appliesToUser(ruleUsers, ruleGroups util.StringSet, user user.Info) bool {
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
