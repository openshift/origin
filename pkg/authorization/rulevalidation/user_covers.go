package rulevalidation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationinterfaces "github.com/openshift/origin/pkg/authorization/interfaces"
)

func ConfirmNoEscalation(ctx kapi.Context, resource unversioned.GroupResource, name string, ruleResolver AuthorizationRuleResolver, role authorizationinterfaces.Role) error {
	ruleResolutionErrors := []error{}

	ownerLocalRules, err := ruleResolver.GetEffectivePolicyRules(ctx)
	if err != nil {
		// do not fail in this case.  Rules are purely additive, so we can continue with a coverage check based on the rules we have
		user, _ := kapi.UserFrom(ctx)
		glog.V(1).Infof("non-fatal error getting local rules for %v: %v", user, err)
		ruleResolutionErrors = append(ruleResolutionErrors, err)
	}
	masterContext := kapi.WithNamespace(ctx, "")
	ownerGlobalRules, err := ruleResolver.GetEffectivePolicyRules(masterContext)
	if err != nil {
		// do not fail in this case.  Rules are purely additive, so we can continue with a coverage check based on the rules we have
		user, _ := kapi.UserFrom(ctx)
		glog.V(1).Infof("non-fatal error getting global rules for %v: %v", user, err)
		ruleResolutionErrors = append(ruleResolutionErrors, err)
	}

	ownerRules := make([]authorizationapi.PolicyRule, 0, len(ownerGlobalRules)+len(ownerLocalRules))
	ownerRules = append(ownerRules, ownerLocalRules...)
	ownerRules = append(ownerRules, ownerGlobalRules...)

	ownerRightsCover, missingRights := Covers(ownerRules, role.Rules())
	if !ownerRightsCover {
		if compactedMissingRights, err := CompactRules(missingRights); err == nil {
			missingRights = compactedMissingRights
		}

		missingRightsStrings := make([]string, 0, len(missingRights))
		for _, missingRight := range missingRights {
			missingRightsStrings = append(missingRightsStrings, missingRight.CompactString())
		}
		sort.Strings(missingRightsStrings)
		user, _ := kapi.UserFrom(ctx)
		var internalErr error
		if len(ruleResolutionErrors) > 0 {
			internalErr = fmt.Errorf("user %q cannot grant extra privileges:\n%v\nrule resolution errors: %v)", user.GetName(), strings.Join(missingRightsStrings, "\n"), ruleResolutionErrors)
		} else {
			internalErr = fmt.Errorf("user %q cannot grant extra privileges:\n%v", user.GetName(), strings.Join(missingRightsStrings, "\n"))
		}
		return kapierrors.NewForbidden(resource, name, internalErr)
	}

	return nil
}
