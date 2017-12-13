package selfsubjectrulesreview

import (
	"fmt"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	rbaclisters "k8s.io/kubernetes/pkg/client/listers/rbac/internalversion"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/apis/authorization/rbacconversion"
	"github.com/openshift/origin/pkg/authorization/registry/subjectrulesreview"
)

type REST struct {
	ruleResolver      rbacregistryvalidation.AuthorizationRuleResolver
	clusterRoleGetter rbaclisters.ClusterRoleLister
}

var _ rest.Creater = &REST{}

func NewREST(ruleResolver rbacregistryvalidation.AuthorizationRuleResolver, clusterRoleGetter rbaclisters.ClusterRoleLister) *REST {
	return &REST{ruleResolver: ruleResolver, clusterRoleGetter: clusterRoleGetter}
}

func (r *REST) New() runtime.Object {
	return &authorizationapi.SelfSubjectRulesReview{}
}

// Create registers a given new ResourceAccessReview instance to r.registry.
func (r *REST) Create(ctx apirequest.Context, obj runtime.Object, _ rest.ValidateObjectFunc, _ bool) (runtime.Object, error) {
	rulesReview, ok := obj.(*authorizationapi.SelfSubjectRulesReview)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a SelfSubjectRulesReview: %#v", obj))
	}
	namespace := apirequest.NamespaceValue(ctx)
	if len(namespace) == 0 {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("namespace is required on this type: %v", namespace))
	}
	callingUser, exists := apirequest.UserFrom(ctx)
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

	rules, errors := subjectrulesreview.GetEffectivePolicyRules(apirequest.WithUser(ctx, userToCheck), r.ruleResolver, r.clusterRoleGetter)

	ret := &authorizationapi.SelfSubjectRulesReview{
		Status: authorizationapi.SubjectRulesReviewStatus{
			Rules: rbacconversion.Convert_rbac_PolicyRules_To_authorization_PolicyRules(rules), //TODO can we fix this ?
		},
	}

	if len(errors) != 0 {
		ret.Status.EvaluationError = kutilerrors.NewAggregate(errors).Error()
	}

	return ret, nil
}
