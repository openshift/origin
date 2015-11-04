package rulevalidation

import (
	"fmt"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationinterfaces "github.com/openshift/origin/pkg/authorization/interfaces"
)

func ConfirmNoEscalation(ctx kapi.Context, ruleResolver AuthorizationRuleResolver, role authorizationinterfaces.Role) error {
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
		user, _ := kapi.UserFrom(ctx)
		return kapierrors.NewUnauthorized(fmt.Sprintf("attempt to grant extra privileges: %v user=%v ownerrules=%v ruleResolutionErrors=%v", missingRights, user, ownerRules, ruleResolutionErrors))
	}

	return nil
}
