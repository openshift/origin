package rulevalidation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"

	authorizationinterfaces "github.com/openshift/origin/pkg/authorization/interfaces"
)

func ConfirmNoEscalation(ctx kapi.Context, resource unversioned.GroupResource, name string, ruleResolver, cachedRuleResolver AuthorizationRuleResolver, role authorizationinterfaces.Role) error {
	var ruleResolutionErrors []error

	user, ok := kapi.UserFrom(ctx)
	if !ok {
		return kapierrors.NewForbidden(resource, name, fmt.Errorf("no user provided in context"))
	}
	namespace, _ := kapi.NamespaceFrom(ctx)

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
