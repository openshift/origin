package authorizer

import (
	"errors"

	"k8s.io/apiserver/pkg/authorization/authorizer"
)

type openshiftAuthorizer struct {
	delegate              authorizer.Authorizer
	forbiddenMessageMaker ForbiddenMessageMaker
}

func NewAuthorizer(delegate authorizer.Authorizer, forbiddenMessageMaker ForbiddenMessageMaker) authorizer.Authorizer {
	return &openshiftAuthorizer{delegate: delegate, forbiddenMessageMaker: forbiddenMessageMaker}
}

func (a *openshiftAuthorizer) Authorize(attributes authorizer.Attributes) (authorizer.Decision, string, error) {
	if attributes.GetUser() == nil {
		return authorizer.DecisionNoOpinion, "", errors.New("no user available on context")
	}
	authorizationDecision, delegateReason, err := a.delegate.Authorize(attributes)
	if authorizationDecision == authorizer.DecisionAllow {
		return authorizer.DecisionAllow, reason(attributes), nil
	}
	// errors are allowed to occur
	if err != nil {
		return authorizationDecision, "", err
	}

	denyReason, err := a.forbiddenMessageMaker.MakeMessage(attributes)
	if err != nil {
		denyReason = err.Error()
	}
	if len(delegateReason) > 0 {
		denyReason += ": " + delegateReason
	}

	return authorizationDecision, denyReason, nil
}

func reason(attributes authorizer.Attributes) string {
	if len(attributes.GetNamespace()) == 0 {
		return "allowed by cluster rule"
	}
	// not 100% accurate, because the rule may have been provided by a cluster rule. we no longer have
	// this distinction upstream in practice.
	return "allowed by openshift authorizer"
}
