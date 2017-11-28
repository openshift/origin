package authorizer

import (
	"errors"

	"k8s.io/apiserver/pkg/authorization/authorizer"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
)

type openshiftAuthorizer struct {
	delegate              kauthorizer.Authorizer
	forbiddenMessageMaker ForbiddenMessageMaker
}

func NewAuthorizer(delegate kauthorizer.Authorizer, forbiddenMessageMaker ForbiddenMessageMaker) authorizer.Authorizer {
	return &openshiftAuthorizer{delegate: delegate, forbiddenMessageMaker: forbiddenMessageMaker}
}

func (a *openshiftAuthorizer) Authorize(attributes authorizer.Attributes) (bool, string, error) {
	if attributes.GetUser() == nil {
		return false, "", errors.New("no user available on context")
	}
	allowed, delegateReason, err := a.delegate.Authorize(attributes)
	if allowed {
		return true, reason(attributes), nil
	}
	// errors are allowed to occur
	if err != nil {
		return false, "", err
	}

	denyReason, err := a.forbiddenMessageMaker.MakeMessage(attributes)
	if err != nil {
		denyReason = err.Error()
	}
	if len(delegateReason) > 0 {
		denyReason += ": " + delegateReason
	}

	return false, denyReason, nil
}

func reason(attributes authorizer.Attributes) string {
	if len(attributes.GetNamespace()) == 0 {
		return "allowed by cluster rule"
	}
	// not 100% accurate, because the rule may have been provided by a cluster rule. we no longer have
	// this distinction upstream in practice.
	return "allowed by openshift authorizer"
}
