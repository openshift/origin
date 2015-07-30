package authorizer

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type openshiftAuthorizer struct {
	ruleResolver          rulevalidation.AuthorizationRuleResolver
	forbiddenMessageMaker ForbiddenMessageMaker
}

func NewAuthorizer(ruleResolver rulevalidation.AuthorizationRuleResolver, forbiddenMessageMaker ForbiddenMessageMaker) Authorizer {
	return &openshiftAuthorizer{ruleResolver, forbiddenMessageMaker}
}

func (a *openshiftAuthorizer) Authorize(ctx kapi.Context, passedAttributes AuthorizationAttributes) (bool, string, error) {
	attributes := coerceToDefaultAuthorizationAttributes(passedAttributes)

	// keep track of errors in case we are unable to authorize the action.
	// It is entirely possible to get an error and be able to continue determine authorization status in spite of it.
	// This is most common when a bound role is missing, but enough roles are still present and bound to authorize the request.
	errs := []error{}

	masterContext := kapi.WithNamespace(ctx, kapi.NamespaceNone)
	globalAllowed, globalReason, err := a.authorizeWithNamespaceRules(masterContext, attributes)
	if globalAllowed {
		return true, globalReason, nil
	}
	if err != nil {
		errs = append(errs, err)
	}

	namespace, _ := kapi.NamespaceFrom(ctx)
	if len(namespace) != 0 {
		namespaceAllowed, namespaceReason, err := a.authorizeWithNamespaceRules(ctx, attributes)
		if namespaceAllowed {
			return true, namespaceReason, nil
		}
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return false, "", kerrors.NewAggregate(errs)
	}

	user, _ := kapi.UserFrom(ctx)
	denyReason, err := a.forbiddenMessageMaker.MakeMessage(MessageContext{user, namespace, attributes})
	if err != nil {
		denyReason = err.Error()
	}

	return false, denyReason, nil
}

func (a *openshiftAuthorizer) GetAllowedSubjects(ctx kapi.Context, attributes AuthorizationAttributes) (util.StringSet, util.StringSet, error) {
	masterContext := kapi.WithNamespace(ctx, kapi.NamespaceNone)
	globalUsers, globalGroups, err := a.getAllowedSubjectsFromNamespaceBindings(masterContext, attributes)
	if err != nil {
		return nil, nil, err
	}
	localUsers, localGroups, err := a.getAllowedSubjectsFromNamespaceBindings(ctx, attributes)
	if err != nil {
		return nil, nil, err
	}

	users := util.StringSet{}
	users.Insert(globalUsers.List()...)
	users.Insert(localUsers.List()...)

	groups := util.StringSet{}
	groups.Insert(globalGroups.List()...)
	groups.Insert(localGroups.List()...)

	return users, groups, nil
}

func (a *openshiftAuthorizer) getAllowedSubjectsFromNamespaceBindings(ctx kapi.Context, passedAttributes AuthorizationAttributes) (util.StringSet, util.StringSet, error) {
	attributes := coerceToDefaultAuthorizationAttributes(passedAttributes)

	roleBindings, err := a.ruleResolver.GetRoleBindings(ctx)
	if err != nil {
		return nil, nil, err
	}

	users := util.StringSet{}
	groups := util.StringSet{}
	for _, roleBinding := range roleBindings {
		role, err := a.ruleResolver.GetRole(roleBinding)
		if err != nil {
			return nil, nil, err
		}

		for _, rule := range role.Rules() {
			matches, err := attributes.RuleMatches(rule)
			if err != nil {
				return nil, nil, err
			}

			if matches {
				users.Insert(roleBinding.Users().List()...)
				groups.Insert(roleBinding.Groups().List()...)
			}
		}
	}

	return users, groups, nil
}

// authorizeWithNamespaceRules returns isAllowed, reason, and error.  If an error is returned, isAllowed and reason are still valid.  This seems strange
// but errors are not always fatal to the authorization process.  It is entirely possible to get an error and be able to continue determine authorization
// status in spite of it.  This is most common when a bound role is missing, but enough roles are still present and bound to authorize the request.
func (a *openshiftAuthorizer) authorizeWithNamespaceRules(ctx kapi.Context, passedAttributes AuthorizationAttributes) (bool, string, error) {
	attributes := coerceToDefaultAuthorizationAttributes(passedAttributes)

	allRules, ruleRetrievalError := a.ruleResolver.GetEffectivePolicyRules(ctx)

	for _, rule := range allRules {
		matches, err := attributes.RuleMatches(rule)
		if err != nil {
			return false, "", err
		}
		if matches {
			namespace := kapi.NamespaceValue(ctx)
			if len(namespace) == 0 {
				return true, "allowed by cluster rule", nil
			}
			return true, "allowed by rule in " + namespace, nil
		}
	}

	return false, "", ruleRetrievalError
}

// TODO this may or may not be the behavior we want for managing rules.  As a for instance, a verb might be specified
// that our attributes builder will never satisfy.  For now, I think gets us close.  Maybe a warning message of some kind?
func coerceToDefaultAuthorizationAttributes(passedAttributes AuthorizationAttributes) *DefaultAuthorizationAttributes {
	attributes, ok := passedAttributes.(*DefaultAuthorizationAttributes)
	if !ok {
		attributes = &DefaultAuthorizationAttributes{
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

func doesApplyToUser(ruleUsers, ruleGroups util.StringSet, user user.Info) bool {
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
