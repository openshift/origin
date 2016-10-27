package subjectrulesreview

import (
	"fmt"
	"sort"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/auth/user"
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
	return &authorizationapi.SubjectRulesReview{}
}

// Create registers a given new ResourceAccessReview instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	rulesReview, ok := obj.(*authorizationapi.SubjectRulesReview)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a SubjectRulesReview: %#v", obj))
	}
	namespace := kapi.NamespaceValue(ctx)
	if len(namespace) == 0 {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("namespace is required on this type: %v", namespace))
	}

	userToCheck := &user.DefaultInfo{
		Name:   rulesReview.Spec.User,
		Groups: rulesReview.Spec.Groups,
		Extra:  map[string][]string{},
	}
	if len(rulesReview.Spec.Scopes) > 0 {
		userToCheck.Extra[authorizationapi.ScopesKey] = rulesReview.Spec.Scopes
	}

	rules, errors := GetEffectivePolicyRules(kapi.WithUser(ctx, userToCheck), r.ruleResolver, r.clusterPolicyGetter)

	ret := &authorizationapi.SubjectRulesReview{
		Status: authorizationapi.SubjectRulesReviewStatus{
			Rules: rules,
		},
	}

	if len(errors) != 0 {
		ret.Status.EvaluationError = kutilerrors.NewAggregate(errors).Error()
	}

	return ret, nil
}

func GetEffectivePolicyRules(ctx kapi.Context, ruleResolver rulevalidation.AuthorizationRuleResolver, clusterPolicyGetter client.ClusterPolicyLister) ([]authorizationapi.PolicyRule, []error) {
	namespace := kapi.NamespaceValue(ctx)
	if len(namespace) == 0 {
		return nil, []error{kapierrors.NewBadRequest(fmt.Sprintf("namespace is required on this type: %v", namespace))}
	}
	user, exists := kapi.UserFrom(ctx)
	if !exists {
		return nil, []error{kapierrors.NewBadRequest(fmt.Sprintf("user missing from context"))}
	}

	var errors []error
	var rules []authorizationapi.PolicyRule
	namespaceRules, err := ruleResolver.RulesFor(user, namespace)
	if err != nil {
		errors = append(errors, err)
	}
	for _, rule := range namespaceRules {
		rules = append(rules, rulevalidation.BreakdownRule(rule)...)
	}

	if scopes := user.GetExtra()[authorizationapi.ScopesKey]; len(scopes) > 0 {
		rules, err = filterRulesByScopes(rules, scopes, namespace, clusterPolicyGetter)
		if err != nil {
			return nil, []error{kapierrors.NewInternalError(err)}
		}
	}

	if compactedRules, err := rulevalidation.CompactRules(rules); err == nil {
		rules = compactedRules
	}
	sort.Sort(authorizationapi.SortableRuleSlice(rules))

	return rules, errors
}

func filterRulesByScopes(rules []authorizationapi.PolicyRule, scopes []string, namespace string, clusterPolicyGetter client.ClusterPolicyLister) ([]authorizationapi.PolicyRule, error) {
	scopeRules, err := scope.ScopesToRules(scopes, namespace, clusterPolicyGetter)
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
