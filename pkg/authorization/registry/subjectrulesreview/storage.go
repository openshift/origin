package subjectrulesreview

import (
	"context"
	"fmt"
	"sort"

	rbacv1 "k8s.io/api/rbac/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	rbaclisters "k8s.io/client-go/listers/rbac/v1"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/apis/authorization/rbacconversion"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
)

type REST struct {
	ruleResolver      rbacregistryvalidation.AuthorizationRuleResolver
	clusterRoleGetter rbaclisters.ClusterRoleLister
}

var _ rest.Creater = &REST{}
var _ rest.Scoper = &REST{}

func NewREST(ruleResolver rbacregistryvalidation.AuthorizationRuleResolver, clusterRoleGetter rbaclisters.ClusterRoleLister) *REST {
	return &REST{ruleResolver: ruleResolver, clusterRoleGetter: clusterRoleGetter}
}

func (r *REST) New() runtime.Object {
	return &authorizationapi.SubjectRulesReview{}
}

func (s *REST) NamespaceScoped() bool {
	return true
}

// Create registers a given new ResourceAccessReview instance to r.registry.
func (r *REST) Create(ctx context.Context, obj runtime.Object, _ rest.ValidateObjectFunc, _ bool) (runtime.Object, error) {
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

	rules, errors := GetEffectivePolicyRules(apirequest.WithUser(ctx, userToCheck), r.ruleResolver, r.clusterRoleGetter)

	ret := &authorizationapi.SubjectRulesReview{
		Status: authorizationapi.SubjectRulesReviewStatus{
			Rules: rbacconversion.Convert_rbacv1_PolicyRules_To_authorization_PolicyRules(rules), //TODO can we fix this ?
		},
	}

	if len(errors) != 0 {
		ret.Status.EvaluationError = kutilerrors.NewAggregate(errors).Error()
	}

	return ret, nil
}

func GetEffectivePolicyRules(ctx context.Context, ruleResolver rbacregistryvalidation.AuthorizationRuleResolver, clusterRoleGetter rbaclisters.ClusterRoleLister) ([]rbacv1.PolicyRule, []error) {
	namespace := apirequest.NamespaceValue(ctx)
	if len(namespace) == 0 {
		return nil, []error{kapierrors.NewBadRequest(fmt.Sprintf("namespace is required on this type: %v", namespace))}
	}
	user, exists := apirequest.UserFrom(ctx)
	if !exists {
		return nil, []error{kapierrors.NewBadRequest(fmt.Sprintf("user missing from context"))}
	}

	var errors []error
	var rules []rbacv1.PolicyRule
	namespaceRules, err := ruleResolver.RulesFor(user, namespace)
	if err != nil {
		errors = append(errors, err)
	}
	for _, rule := range namespaceRules {
		rules = append(rules, rbacregistryvalidation.BreakdownRule(rule)...)
	}

	if scopes := user.GetExtra()[authorizationapi.ScopesKey]; len(scopes) > 0 {
		rules, err = filterRulesByScopes(rules, scopes, namespace, clusterRoleGetter)
		if err != nil {
			return nil, []error{kapierrors.NewInternalError(err)}
		}
	}

	if compactedRules, err := rbacregistryvalidation.CompactRules(rules); err == nil {
		rules = compactedRules
	}
	sort.Sort(rbacv1helpers.SortableRuleSlice(rules))

	return rules, errors
}

func filterRulesByScopes(rules []rbacv1.PolicyRule, scopes []string, namespace string, clusterRoleGetter rbaclisters.ClusterRoleLister) ([]rbacv1.PolicyRule, error) {
	scopeRules, err := scope.ScopesToRules(scopes, namespace, clusterRoleGetter)
	if err != nil {
		return nil, err
	}

	filteredRules := []rbacv1.PolicyRule{}
	for _, rule := range rules {
		if allowed, _ := rbacregistryvalidation.Covers(scopeRules, []rbacv1.PolicyRule{rule}); allowed {
			filteredRules = append(filteredRules, rule)
		}
	}

	return filteredRules, nil
}
