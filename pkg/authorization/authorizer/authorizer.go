package authorizer

import (
	"errors"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type openshiftAuthorizer struct {
	ruleResolver          rulevalidation.AuthorizationRuleResolver
	forbiddenMessageMaker ForbiddenMessageMaker
}

func NewAuthorizer(ruleResolver rulevalidation.AuthorizationRuleResolver, forbiddenMessageMaker ForbiddenMessageMaker) Authorizer {
	return &openshiftAuthorizer{ruleResolver, forbiddenMessageMaker}
}

func (a *openshiftAuthorizer) Authorize(ctx kapi.Context, passedAttributes Action) (bool, string, error) {
	attributes := CoerceToDefaultAuthorizationAttributes(passedAttributes)

	user, ok := kapi.UserFrom(ctx)
	if !ok {
		return false, "", errors.New("no user available on context")
	}
	namespace, _ := kapi.NamespaceFrom(ctx)
	allowed, reason, err := a.authorizeWithNamespaceRules(user, namespace, attributes)
	if allowed {
		return true, reason, nil
	}
	// errors are allowed to occur
	if err != nil {
		return false, "", err
	}

	denyReason, err := a.forbiddenMessageMaker.MakeMessage(MessageContext{user, namespace, attributes})
	if err != nil {
		denyReason = err.Error()
	}

	return false, denyReason, nil
}

// GetAllowedSubjects returns the subjects it knows can perform the action.
// If we got an error, then the list of subjects may not be complete, but it does not contain any incorrect names.
// This is done because policy rules are purely additive and policy determinations
// can be made on the basis of those rules that are found.
func (a *openshiftAuthorizer) GetAllowedSubjects(ctx kapi.Context, attributes Action) (sets.String, sets.String, error) {
	namespace, _ := kapi.NamespaceFrom(ctx)
	return a.getAllowedSubjectsFromNamespaceBindings(namespace, attributes)
}

func (a *openshiftAuthorizer) getAllowedSubjectsFromNamespaceBindings(namespace string, passedAttributes Action) (sets.String, sets.String, error) {
	attributes := CoerceToDefaultAuthorizationAttributes(passedAttributes)

	var errs []error

	roleBindings, err := a.ruleResolver.GetRoleBindings(namespace)
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
			matches, err := attributes.RuleMatches(rule)
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
func (a *openshiftAuthorizer) authorizeWithNamespaceRules(user user.Info, namespace string, passedAttributes Action) (bool, string, error) {
	attributes := CoerceToDefaultAuthorizationAttributes(passedAttributes)

	allRules, ruleRetrievalError := a.ruleResolver.RulesFor(user, namespace)

	var errs []error
	for _, rule := range allRules {
		matches, err := attributes.RuleMatches(rule)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if matches {
			if len(namespace) == 0 {
				return true, "allowed by cluster rule", nil
			}
			// not 100% accurate, because the rule may have been provided by a cluster rule. we no longer have
			// this distinction upstream in practice.
			return true, "allowed by rule in " + namespace, nil
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

// TODO this may or may not be the behavior we want for managing rules.  As a for instance, a verb might be specified
// that our attributes builder will never satisfy.  For now, I think gets us close.  Maybe a warning message of some kind?
func CoerceToDefaultAuthorizationAttributes(passedAttributes Action) *DefaultAuthorizationAttributes {
	attributes, ok := passedAttributes.(*DefaultAuthorizationAttributes)
	if !ok {
		attributes = &DefaultAuthorizationAttributes{
			APIGroup:          passedAttributes.GetAPIGroup(),
			Verb:              passedAttributes.GetVerb(),
			RequestAttributes: passedAttributes.GetRequestAttributes(),
			Resource:          passedAttributes.GetResource(),
			ResourceName:      passedAttributes.GetResourceName(),
			NonResourceURL:    passedAttributes.IsNonResourceURL(),
			URL:               passedAttributes.GetURL(),
		}
	}

	return attributes
}

func doesApplyToUser(ruleUsers, ruleGroups sets.String, user user.Info) bool {
	if ruleUsers.Has(user.GetName()) {
		return true
	}

	for _, currGroup := range user.GetGroups() {
		if ruleGroups.Has(currGroup) {
			return true
		}
	}

	return false
}
