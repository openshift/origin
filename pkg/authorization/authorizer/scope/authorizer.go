package scope

import (
	"fmt"

	"k8s.io/kubernetes/pkg/auth/authorizer"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	defaultauthorizer "github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/client"
)

type scopeAuthorizer struct {
	delegate            defaultauthorizer.Authorizer
	clusterPolicyGetter client.ClusterPolicyLister

	forbiddenMessageMaker defaultauthorizer.ForbiddenMessageMaker
}

func NewAuthorizer(delegate defaultauthorizer.Authorizer, clusterPolicyGetter client.ClusterPolicyLister, forbiddenMessageMaker defaultauthorizer.ForbiddenMessageMaker) defaultauthorizer.Authorizer {
	return &scopeAuthorizer{delegate: delegate, clusterPolicyGetter: clusterPolicyGetter, forbiddenMessageMaker: forbiddenMessageMaker}
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
	rules, err := ScopesToRules(scopes, attributes.GetNamespace(), a.clusterPolicyGetter)
	if err != nil {
		nonFatalErrors = append(nonFatalErrors, err)
	}

	for _, rule := range rules {
		// check rule against attributes
		matches, err := defaultauthorizer.RuleMatches(attributes, rule)
		if err != nil {
			nonFatalErrors = append(nonFatalErrors, err)
			continue
		}
		if matches {
			return a.delegate.Authorize(attributes)
		}
	}

	denyReason, err := a.forbiddenMessageMaker.MakeMessage(defaultauthorizer.MessageContext{Attributes: attributes})
	if err != nil {
		denyReason = err.Error()
	}

	return false, fmt.Sprintf("scopes %v prevent this action; %v", scopes, denyReason), kerrors.NewAggregate(nonFatalErrors)
}

// TODO remove this. We don't logically need it, but it requires splitting our interface
// GetAllowedSubjects returns the subjects it knows can perform the action.
func (a *scopeAuthorizer) GetAllowedSubjects(attributes authorizer.Attributes) (sets.String, sets.String, error) {
	return a.delegate.GetAllowedSubjects(attributes)
}
