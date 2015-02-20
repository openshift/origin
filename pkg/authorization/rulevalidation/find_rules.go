package rulevalidation

import (
	"errors"
	"fmt"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type DefaultRuleResolver struct {
	policyGetter  PolicyGetter
	bindingLister BindingLister
}

func NewDefaultRuleResolver(policyGetter PolicyGetter, bindingLister BindingLister) *DefaultRuleResolver {
	return &DefaultRuleResolver{policyGetter, bindingLister}
}

type AuthorizationRuleResolver interface {
	GetRoleBindings(ctx kapi.Context) ([]authorizationapi.RoleBinding, error)
	GetRole(roleBinding authorizationapi.RoleBinding) (*authorizationapi.Role, error)
	// getEffectivePolicyRules returns the list of rules that apply to a given user in a given namespace and error.  If an error is returned, the slice of
	// PolicyRules may not be complete, but it contains all retrievable rules.  This is done because policy rules are purely additive and policy determinations
	// can be made on the basis of those rules that are found.
	GetEffectivePolicyRules(ctx kapi.Context) ([]authorizationapi.PolicyRule, error)
}

type PolicyGetter interface {
	// GetPolicy retrieves a specific policy.
	GetPolicy(ctx kapi.Context, id string) (*authorizationapi.Policy, error)
}

type BindingLister interface {
	// ListPolicyBindings obtains list of policyBindings that match a selector.
	ListPolicyBindings(ctx kapi.Context, labels, fields klabels.Selector) (*authorizationapi.PolicyBindingList, error)
}

// getPolicy provides a point for easy caching
func (a *DefaultRuleResolver) getPolicy(ctx kapi.Context) (*authorizationapi.Policy, error) {
	policy, err := a.policyGetter.GetPolicy(ctx, authorizationapi.PolicyName)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, err
	}

	return policy, nil
}

// getPolicyBindings provides a point for easy caching
func (a *DefaultRuleResolver) getPolicyBindings(ctx kapi.Context) ([]authorizationapi.PolicyBinding, error) {
	policyBindingList, err := a.bindingLister.ListPolicyBindings(ctx, klabels.Everything(), klabels.Everything())
	if err != nil {
		return nil, err
	}

	return policyBindingList.Items, nil
}

// getRoleBindings provides a point for easy caching
func (a *DefaultRuleResolver) GetRoleBindings(ctx kapi.Context) ([]authorizationapi.RoleBinding, error) {
	policyBindings, err := a.getPolicyBindings(ctx)
	if err != nil {
		return nil, err
	}

	ret := make([]authorizationapi.RoleBinding, 0, len(policyBindings))
	for _, policyBinding := range policyBindings {
		for _, value := range policyBinding.RoleBindings {
			ret = append(ret, value)
		}
	}

	return ret, nil
}

func (a *DefaultRuleResolver) GetRole(roleBinding authorizationapi.RoleBinding) (*authorizationapi.Role, error) {
	namespace := roleBinding.RoleRef.Namespace
	name := roleBinding.RoleRef.Name

	ctx := kapi.WithNamespace(kapi.NewContext(), namespace)
	policy, err := a.getPolicy(ctx)
	if err != nil {
		return nil, err
	}

	role, exists := policy.Roles[name]
	if !exists {
		return nil, fmt.Errorf("role %#v not found", roleBinding.RoleRef)
	}

	return &role, nil
}

// getEffectivePolicyRules returns the list of rules that apply to a given user in a given namespace and error.  If an error is returned, the slice of
// PolicyRules may not be complete, but it contains all retrievable rules.  This is done because policy rules are purely additive and policy determinations
// can be made on the basis of those rules that are found.
func (a *DefaultRuleResolver) GetEffectivePolicyRules(ctx kapi.Context) ([]authorizationapi.PolicyRule, error) {
	roleBindings, err := a.GetRoleBindings(ctx)
	if err != nil {
		return nil, err
	}

	errs := []error{}
	rules := make([]authorizationapi.PolicyRule, 0, len(roleBindings))
	for _, roleBinding := range roleBindings {
		role, err := a.GetRole(roleBinding)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, curr := range role.Rules {
			user, exists := kapi.UserFrom(ctx)
			if !exists {
				errs = append(errs, errors.New("user missing from context"))
			}

			if doesApplyToUser(roleBinding.Users, roleBinding.Groups, user) {
				rules = append(rules, curr)
			}
		}
	}

	return rules, kerrors.NewAggregate(errs)
}

func doesApplyToUser(ruleUsers, ruleGroups util.StringSet, user user.Info) bool {
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
