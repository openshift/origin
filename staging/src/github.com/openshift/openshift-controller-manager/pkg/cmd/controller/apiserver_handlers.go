package controller

import (
	"k8s.io/apimachinery/pkg/util/sets"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
)

type bypassAuthorizer struct {
	paths      sets.String
	authorizer kauthorizer.Authorizer
}

// newBypassAuthorizer creates an Authorizer that always allows the exact paths described, and delegates to the nested
// authorizer for everything else.
func newBypassAuthorizer(auth kauthorizer.Authorizer, paths ...string) kauthorizer.Authorizer {
	return bypassAuthorizer{paths: sets.NewString(paths...), authorizer: auth}
}

func (a bypassAuthorizer) Authorize(attributes kauthorizer.Attributes) (allowed kauthorizer.Decision, reason string, err error) {
	if !attributes.IsResourceRequest() && a.paths.Has(attributes.GetPath()) {
		return kauthorizer.DecisionAllow, "always allowed", nil
	}
	return a.authorizer.Authorize(attributes)
}
