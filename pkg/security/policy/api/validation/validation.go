package validation

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/security/policy/api"
)

// ValidatePodSecurityPolicyName can be used to check whether the given
// security context constraint name is valid.
// Prefix indicates this name will be used as part of generation, in which case
// trailing dashes are allowed.
func ValidatePodSecurityPolicyName(name string, prefix bool) (bool, string) {
	return validation.NameIsDNSSubdomain(name, prefix)
}

// ValidatePodSecurityPolicy validates a PodSecurityPolicy.
func ValidatePodSecurityPolicy(psp *api.PodSecurityPolicy) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMeta(&psp.ObjectMeta, false, ValidatePodSecurityPolicyName).Prefix("metadata")...)
	allErrs = append(allErrs, validatePodSecurityPolicySpec(psp).Prefix("spec")...)
	return allErrs
}

func validatePodSecurityPolicySpec(psp *api.PodSecurityPolicy) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	spec := psp.Spec

	// ensure the user strat has a valid type
	switch spec.RunAsUser.Type {
	case api.RunAsUserStrategyMustRunAs, api.RunAsUserStrategyMustRunAsNonRoot, api.RunAsUserStrategyRunAsAny, api.RunAsUserStrategyMustRunAsRange:
	//good types
	default:
		msg := fmt.Sprintf("invalid strategy type; valid values are %s, %s, %s", api.RunAsUserStrategyMustRunAs, api.RunAsUserStrategyMustRunAsNonRoot, api.RunAsUserStrategyRunAsAny)
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("runAsUser.type", spec.RunAsUser.Type, msg))
	}

	// if specified, uid cannot be negative
	if spec.RunAsUser.UID != nil {
		if *spec.RunAsUser.UID < 0 {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("runAsUser.uid", *spec.RunAsUser.UID, "uid cannot be negative"))
		}
	}

	// ensure the selinux strat has a valid type
	switch spec.SELinuxContext.Type {
	case api.SELinuxStrategyMustRunAs, api.SELinuxStrategyRunAsAny:
	//good types
	default:
		msg := fmt.Sprintf("invalid strategy type; valid values are %s, %s", api.SELinuxStrategyMustRunAs, api.SELinuxStrategyRunAsAny)
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("seLinuxContext.type", spec.RunAsUser.Type, msg))
	}
	return allErrs
}

// ValidateSecurityContextConstraints validates a SecurityContextConstraints for updates.
func ValidatePodSecurityPolicyUpdate(old *api.PodSecurityPolicy, new *api.PodSecurityPolicy) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&old.ObjectMeta, &new.ObjectMeta).Prefix("metadata")...)
	allErrs = append(allErrs, ValidatePodSecurityPolicy(new)...)
	return allErrs
}
