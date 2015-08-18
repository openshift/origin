package validation

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/security/scc/api"
)

// ValidateSecurityContextConstraintsName can be used to check whether the given
// security context constraint name is valid.
// Prefix indicates this name will be used as part of generation, in which case
// trailing dashes are allowed.
func ValidateSecurityContextConstraintsName(name string, prefix bool) (bool, string) {
	return validation.NameIsDNSSubdomain(name, prefix)
}

// ValidateSecurityContextConstraints validates a SecurityContextConstraints.
func ValidateSecurityContextConstraints(scc *api.SecurityContextConstraints) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMeta(&scc.ObjectMeta, false, ValidateSecurityContextConstraintsName).Prefix("metadata")...)

	// ensure the user strat has a valid type
	switch scc.RunAsUser.Type {
	case api.RunAsUserStrategyMustRunAs, api.RunAsUserStrategyMustRunAsNonRoot, api.RunAsUserStrategyRunAsAny, api.RunAsUserStrategyMustRunAsRange:
	//good types
	default:
		msg := fmt.Sprintf("invalid strategy type.  Valid values are %s, %s, %s", api.RunAsUserStrategyMustRunAs, api.RunAsUserStrategyMustRunAsNonRoot, api.RunAsUserStrategyRunAsAny)
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("runAsUser.type", scc.RunAsUser.Type, msg))
	}

	// if specified, uid cannot be negative
	if scc.RunAsUser.UID != nil {
		if *scc.RunAsUser.UID < 0 {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("runAsUser.uid", *scc.RunAsUser.UID, "uid cannot be negative"))
		}
	}

	// ensure the selinux strat has a valid type
	switch scc.SELinuxContext.Type {
	case api.SELinuxStrategyMustRunAs, api.SELinuxStrategyRunAsAny:
	//good types
	default:
		msg := fmt.Sprintf("invalid strategy type.  Valid values are %s, %s", api.SELinuxStrategyMustRunAs, api.SELinuxStrategyRunAsAny)
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("seLinuxContext.type", scc.RunAsUser.Type, msg))
	}

	return allErrs
}

// ValidateSecurityContextConstraints validates a SecurityContextConstraints for updates.
func ValidateSecurityContextConstraintsUpdate(old *api.SecurityContextConstraints, new *api.SecurityContextConstraints) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&old.ObjectMeta, &new.ObjectMeta).Prefix("metadata")...)
	allErrs = append(allErrs, ValidateSecurityContextConstraints(new)...)
	return allErrs
}
