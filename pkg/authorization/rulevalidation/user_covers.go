package rulevalidation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	authorizationinterfaces "github.com/openshift/origin/pkg/authorization/interfaces"
)

func ConfirmNoEscalation(ctx apirequest.Context, resource schema.GroupResource, name string, ruleResolver, cachedRuleResolver AuthorizationRuleResolver, role authorizationinterfaces.Role) error {
	var ruleResolutionErrors []error

	user, ok := apirequest.UserFrom(ctx)
	if !ok {
		return kapierrors.NewForbidden(resource, name, fmt.Errorf("no user provided in context"))
	}
	namespace, _ := apirequest.NamespaceFrom(ctx)

	// if a cached resolver is provided, attempt to verify coverage against the cache, then fall back to the normal
	// path otherwise
	if cachedRuleResolver != nil {
		if ownerRules, err := cachedRuleResolver.RulesFor(user, namespace); err == nil {
			if ownerRightsCover, _ := Covers(ownerRules, role.Rules()); ownerRightsCover {
				return nil
			}
		}
	}

	ownerRules, err := ruleResolver.RulesFor(user, namespace)
	if err != nil {
		// do not fail in this case.  Rules are purely additive, so we can continue with a coverage check based on the rules we have
		glog.V(1).Infof("non-fatal error getting rules for %v: %v", user, err)
		ruleResolutionErrors = append(ruleResolutionErrors, err)
	}

	ownerRightsCover, missingRights := Covers(ownerRules, role.Rules())
	if ownerRightsCover {
		return nil
	}

	// determine what resources the user is missing
	if compactedMissingRights, err := CompactRules(missingRights); err == nil {
		missingRights = compactedMissingRights
	}

	missingRightsStrings := make([]string, 0, len(missingRights))
	for _, missingRight := range missingRights {
		missingRightsStrings = append(missingRightsStrings, missingRight.CompactString())
	}
	sort.Strings(missingRightsStrings)

	var internalErr error
	if len(ruleResolutionErrors) > 0 {
		internalErr = fmt.Errorf("user %q cannot grant extra privileges:\n%v\nrule resolution errors: %v)", user.GetName(), strings.Join(missingRightsStrings, "\n"), ruleResolutionErrors)
	} else {
		internalErr = fmt.Errorf("user %q cannot grant extra privileges:\n%v", user.GetName(), strings.Join(missingRightsStrings, "\n"))
	}
	return kapierrors.NewForbidden(resource, name, internalErr)
}

// EscalationAllowed returns true if a particular user is allowed to escalate his powers
func EscalationAllowed(ctx apirequest.Context) bool {
	u, ok := apirequest.UserFrom(ctx)
	if !ok {
		return false
	}

	// system:masters is special because the API server uses it for privileged loopback connections
	// therefore we know that a member of system:masters can always do anything
	for _, group := range u.GetGroups() {
		if group == user.SystemPrivilegedGroup {
			return true
		}
	}

	return false
}
