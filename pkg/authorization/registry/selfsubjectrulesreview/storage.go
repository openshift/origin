package selfsubjectrulesreview

import (
	"fmt"
	"sort"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/runtime"
	kutilerrors "k8s.io/kubernetes/pkg/util/errors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	"github.com/openshift/origin/pkg/client"
)

type REST struct {
	ruleResolver        rulevalidation.AuthorizationRuleResolver
	clusterPolicyGetter client.ClusterPolicyLister
}

func NewREST(ruleResolver rulevalidation.AuthorizationRuleResolver, clusterPolicyGetter client.ClusterPolicyLister) *REST {
	return &REST{ruleResolver: ruleResolver, clusterPolicyGetter: clusterPolicyGetter}
}

func (r *REST) New() runtime.Object {
	return &authorizationapi.SelfSubjectRulesReview{}
}

// Create registers a given new ResourceAccessReview instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	rulesReview, ok := obj.(*authorizationapi.SelfSubjectRulesReview)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a SelfSubjectRulesReview: %#v", obj))
	}
	namespace := kapi.NamespaceValue(ctx)
	if len(namespace) == 0 {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("namespace is required on this type: %v", namespace))
	}
	user, exists := kapi.UserFrom(ctx)
	if !exists {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("user missing from context"))
	}

	errors := []error{}

	rules := []authorizationapi.PolicyRule{}
	namespaceRules, err := r.ruleResolver.GetEffectivePolicyRules(ctx)
	if err != nil {
		errors = append(errors, err)
	}
	for _, rule := range namespaceRules {
		rules = append(rules, rulevalidation.BreakdownRule(rule)...)
	}
	if len(namespace) != 0 {
		masterContext := kapi.WithNamespace(ctx, kapi.NamespaceNone)
		clusterRules, err := r.ruleResolver.GetEffectivePolicyRules(masterContext)
		if err != nil {
			errors = append(errors, err)
		}
		for _, rule := range clusterRules {
			rules = append(rules, rulevalidation.BreakdownRule(rule)...)
		}
	}

	switch {
	case rulesReview.Spec.Scopes == nil:
		if scopes, _ := user.GetExtra()[authorizationapi.ScopesKey]; len(scopes) > 0 {
			rules, err = r.filterRulesByScopes(rules, scopes, namespace)
			if err != nil {
				return nil, kapierrors.NewInternalError(err)
			}
		}

	case len(rulesReview.Spec.Scopes) > 0:
		rules, err = r.filterRulesByScopes(rules, rulesReview.Spec.Scopes, namespace)
		if err != nil {
			return nil, kapierrors.NewInternalError(err)
		}

	}

	if compactedRules, err := rulevalidation.CompactRules(rules); err == nil {
		rules = compactedRules
	}
	sort.Sort(authorizationapi.SortableRuleSlice(rules))

	ret := &authorizationapi.SelfSubjectRulesReview{
		Status: authorizationapi.SubjectRulesReviewStatus{
			Rules: rules,
		},
	}

	if len(errors) != 0 {
		ret.Status.EvaluationError = kutilerrors.NewAggregate(errors).Error()
	}

	return ret, nil
}

func (r *REST) filterRulesByScopes(rules []authorizationapi.PolicyRule, scopes []string, namespace string) ([]authorizationapi.PolicyRule, error) {
	scopeRules, err := scope.ScopesToRules(scopes, namespace, r.clusterPolicyGetter)
	if err != nil {
		return nil, err
	}

	filteredRules := []authorizationapi.PolicyRule{}
	for _, rule := range rules {
		if allowed, _ := rulevalidation.Covers(scopeRules, []authorizationapi.PolicyRule{rule}); allowed {
			filteredRules = append(filteredRules, rule)
		}
	}

	return filteredRules, nil
}
