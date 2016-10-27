package selfsubjectrulesreview

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/runtime"
	kutilerrors "k8s.io/kubernetes/pkg/util/errors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	"github.com/openshift/origin/pkg/authorization/registry/subjectrulesreview"
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
	callingUser, exists := kapi.UserFrom(ctx)
	if !exists {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("user missing from context"))
	}

	// copy the user to avoid mutating the original extra map
	userToCheck := &user.DefaultInfo{
		Name:   callingUser.GetName(),
		Groups: callingUser.GetGroups(),
		Extra:  map[string][]string{},
	}
	switch {
	case rulesReview.Spec.Scopes == nil:
		for k, v := range callingUser.GetExtra() {
			userToCheck.Extra[k] = v
		}

	case len(rulesReview.Spec.Scopes) > 0:
		userToCheck.Extra[authorizationapi.ScopesKey] = rulesReview.Spec.Scopes
	}

	rules, errors := subjectrulesreview.GetEffectivePolicyRules(kapi.WithUser(ctx, userToCheck), r.ruleResolver, r.clusterPolicyGetter)

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
