package subjectrulesreview

import (
	"fmt"
	"sort"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	authorizationlister "github.com/openshift/origin/pkg/authorization/generated/listers/authorization/internalversion"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type REST struct {
	ruleResolver        rulevalidation.AuthorizationRuleResolver
	clusterPolicyGetter authorizationlister.ClusterPolicyLister
}

func NewREST(ruleResolver rulevalidation.AuthorizationRuleResolver, clusterPolicyGetter authorizationlister.ClusterPolicyLister) *REST {
	return &REST{ruleResolver: ruleResolver, clusterPolicyGetter: clusterPolicyGetter}
}

func (r *REST) New() runtime.Object {
	return &authorizationapi.SubjectRulesReview{}
}

// Create registers a given new ResourceAccessReview instance to r.registry.
func (r *REST) Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error) {
	rulesReview, ok := obj.(*authorizationapi.SubjectRulesReview)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a SubjectRulesReview: %#v", obj))
	}
	namespace := apirequest.NamespaceValue(ctx)
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

	rules, errors := GetEffectivePolicyRules(apirequest.WithUser(ctx, userToCheck), r.ruleResolver, r.clusterPolicyGetter)

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

func GetEffectivePolicyRules(ctx apirequest.Context, ruleResolver rulevalidation.AuthorizationRuleResolver, clusterPolicyGetter authorizationlister.ClusterPolicyLister) ([]authorizationapi.PolicyRule, []error) {
	namespace := apirequest.NamespaceValue(ctx)
	if len(namespace) == 0 {
		return nil, []error{kapierrors.NewBadRequest(fmt.Sprintf("namespace is required on this type: %v", namespace))}
	}
	user, exists := apirequest.UserFrom(ctx)
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

func filterRulesByScopes(rules []authorizationapi.PolicyRule, scopes []string, namespace string, clusterPolicyGetter authorizationlister.ClusterPolicyLister) ([]authorizationapi.PolicyRule, error) {
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
