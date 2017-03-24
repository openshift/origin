package authorizer

import (
	"errors"

	"k8s.io/kubernetes/pkg/auth/authorizer"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type openshiftAuthorizer struct {
	ruleResolver          rulevalidation.AuthorizationRuleResolver
	forbiddenMessageMaker ForbiddenMessageMaker
}

func NewAuthorizer(ruleResolver rulevalidation.AuthorizationRuleResolver, forbiddenMessageMaker ForbiddenMessageMaker) (authorizer.Authorizer, SubjectLocator) {
	ret := &openshiftAuthorizer{ruleResolver, forbiddenMessageMaker}
	return ret, ret
}

func (a *openshiftAuthorizer) Authorize(attributes authorizer.Attributes) (bool, string, error) {
	if attributes.GetUser() == nil {
		return false, "", errors.New("no user available on context")
	}
	allowed, reason, err := a.authorizeWithNamespaceRules(attributes)
	if allowed {
		return true, reason, nil
	}
	// errors are allowed to occur
	if err != nil {
		return false, "", err
	}

	denyReason, err := a.forbiddenMessageMaker.MakeMessage(attributes)
	if err != nil {
		denyReason = err.Error()
	}

	return false, denyReason, nil
}

// GetAllowedSubjects returns the subjects it knows can perform the action.
// If we got an error, then the list of subjects may not be complete, but it does not contain any incorrect names.
// This is done because policy rules are purely additive and policy determinations
// can be made on the basis of those rules that are found.
func (a *openshiftAuthorizer) GetAllowedSubjects(attributes authorizer.Attributes) (sets.String, sets.String, error) {
	return a.getAllowedSubjectsFromNamespaceBindings(attributes)
}

func (a *openshiftAuthorizer) getAllowedSubjectsFromNamespaceBindings(attributes authorizer.Attributes) (sets.String, sets.String, error) {
	var errs []error

	roleBindings, err := a.ruleResolver.GetRoleBindings(attributes.GetNamespace())
	if err != nil {
		errs = append(errs, err)
	}

	users := sets.String{}
	groups := sets.String{}
	for _, roleBinding := range roleBindings {
		role, err := a.ruleResolver.GetRole(roleBinding)
		if err != nil {
			// If we got an error, then the list of subjects may not be complete, but it does not contain any incorrect names.
			// This is done because policy rules are purely additive and policy determinations
			// can be made on the basis of those rules that are found.
			errs = append(errs, err)
			continue
		}

		for _, rule := range role.Rules() {
			matches, err := RuleMatches(attributes, rule)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if matches {
				users.Insert(roleBinding.Users().List()...)
				groups.Insert(roleBinding.Groups().List()...)
			}
		}
	}

	return users, groups, kerrors.NewAggregate(errs)
}

// authorizeWithNamespaceRules returns isAllowed, reason, and error.  If an error is returned, isAllowed and reason are still valid.  This seems strange
// but errors are not always fatal to the authorization process.  It is entirely possible to get an error and be able to continue determine authorization
// status in spite of it.  This is most common when a bound role is missing, but enough roles are still present and bound to authorize the request.
func (a *openshiftAuthorizer) authorizeWithNamespaceRules(attributes authorizer.Attributes) (bool, string, error) {
	allRules, ruleRetrievalError := a.ruleResolver.RulesFor(attributes.GetUser(), attributes.GetNamespace())

	var errs []error
	for _, rule := range allRules {
		matches, err := RuleMatches(attributes, rule)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if matches {
			if len(attributes.GetNamespace()) == 0 {
				return true, "allowed by cluster rule", nil
			}
			// not 100% accurate, because the rule may have been provided by a cluster rule. we no longer have
			// this distinction upstream in practice.
			return true, "allowed by rule in " + attributes.GetNamespace(), nil
		}
	}
	if len(errs) == 0 {
		return false, "", ruleRetrievalError
	}
	if ruleRetrievalError != nil {
		errs = append(errs, ruleRetrievalError)
	}
	return false, "", kerrors.NewAggregate(errs)
}
