package scope

import (
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	rbaclisters "k8s.io/kubernetes/pkg/client/listers/rbac/internalversion"
	authorizerrbac "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	defaultauthorizer "github.com/openshift/origin/pkg/authorization/authorizer"
)

type scopeAuthorizer struct {
	delegate          authorizer.Authorizer
	clusterRoleGetter rbaclisters.ClusterRoleLister

	forbiddenMessageMaker defaultauthorizer.ForbiddenMessageMaker
}

func NewAuthorizer(delegate authorizer.Authorizer, clusterRoleGetter rbaclisters.ClusterRoleLister, forbiddenMessageMaker defaultauthorizer.ForbiddenMessageMaker) authorizer.Authorizer {
	return &scopeAuthorizer{delegate: delegate, clusterRoleGetter: clusterRoleGetter, forbiddenMessageMaker: forbiddenMessageMaker}
}

func (a *scopeAuthorizer) Authorize(attributes authorizer.Attributes) (bool, string, error) {
	user := attributes.GetUser()
	if user == nil {
		return false, "", fmt.Errorf("user missing from context")
	}

	scopes := user.GetExtra()[authorizationapi.ScopesKey]
	if len(scopes) == 0 {
		return a.delegate.Authorize(attributes)
	}

	nonFatalErrors := []error{}

	// scopeResolutionErrors aren't fatal.  If any of the scopes we find allow this, then the overall scope limits allow it
	rules, err := ScopesToRules(scopes, attributes.GetNamespace(), a.clusterRoleGetter)
	if err != nil {
		nonFatalErrors = append(nonFatalErrors, err)
	}

	// check rules against attributes
	if authorizerrbac.RulesAllow(attributes, rules...) {
		return a.delegate.Authorize(attributes)
	}

	denyReason, err := a.forbiddenMessageMaker.MakeMessage(attributes)
	if err != nil {
		denyReason = err.Error()
	}

	return false, fmt.Sprintf("scopes %v prevent this action; %v", scopes, denyReason), kerrors.NewAggregate(nonFatalErrors)
}
