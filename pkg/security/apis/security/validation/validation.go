package validation

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapivalidation "k8s.io/kubernetes/pkg/apis/core/validation"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

// ValidateSecurityContextConstraintsName can be used to check whether the given
// security context constraint name is valid.
// Prefix indicates this name will be used as part of generation, in which case
// trailing dashes are allowed.
var ValidateSecurityContextConstraintsName = validation.NameIsDNSSubdomain

func ValidateSecurityContextConstraints(scc *securityapi.SecurityContextConstraints) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&scc.ObjectMeta, false, ValidateSecurityContextConstraintsName, field.NewPath("metadata"))

	if scc.Priority != nil {
		if *scc.Priority < 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("priority"), *scc.Priority, "priority cannot be negative"))
		}
	}

	// ensure the user strat has a valid type
	runAsUserPath := field.NewPath("runAsUser")
	switch scc.RunAsUser.Type {
	case securityapi.RunAsUserStrategyMustRunAs, securityapi.RunAsUserStrategyMustRunAsNonRoot, securityapi.RunAsUserStrategyRunAsAny, securityapi.RunAsUserStrategyMustRunAsRange:
		//good types
	default:
		msg := fmt.Sprintf("invalid strategy type.  Valid values are %s, %s, %s, %s", securityapi.RunAsUserStrategyMustRunAs, securityapi.RunAsUserStrategyMustRunAsNonRoot, securityapi.RunAsUserStrategyMustRunAsRange, securityapi.RunAsUserStrategyRunAsAny)
		allErrs = append(allErrs, field.Invalid(runAsUserPath.Child("type"), scc.RunAsUser.Type, msg))
	}

	// if specified, uid cannot be negative
	if scc.RunAsUser.UID != nil {
		if *scc.RunAsUser.UID < 0 {
			allErrs = append(allErrs, field.Invalid(runAsUserPath.Child("uid"), *scc.RunAsUser.UID, "uid cannot be negative"))
		}
	}

	// ensure the selinux strat has a valid type
	seLinuxContextPath := field.NewPath("seLinuxContext")
	switch scc.SELinuxContext.Type {
	case securityapi.SELinuxStrategyMustRunAs, securityapi.SELinuxStrategyRunAsAny:
		//good types
	default:
		msg := fmt.Sprintf("invalid strategy type.  Valid values are %s, %s", securityapi.SELinuxStrategyMustRunAs, securityapi.SELinuxStrategyRunAsAny)
		allErrs = append(allErrs, field.Invalid(seLinuxContextPath.Child("type"), scc.SELinuxContext.Type, msg))
	}

	// ensure the fsgroup strat has a valid type
	if scc.FSGroup.Type != securityapi.FSGroupStrategyMustRunAs && scc.FSGroup.Type != securityapi.FSGroupStrategyRunAsAny {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("fsGroup", "type"), scc.FSGroup.Type,
			[]string{string(securityapi.FSGroupStrategyMustRunAs), string(securityapi.FSGroupStrategyRunAsAny)}))
	}
	allErrs = append(allErrs, validateIDRanges(scc.FSGroup.Ranges, field.NewPath("fsGroup"))...)

	if scc.SupplementalGroups.Type != securityapi.SupplementalGroupsStrategyMustRunAs &&
		scc.SupplementalGroups.Type != securityapi.SupplementalGroupsStrategyRunAsAny {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("supplementalGroups", "type"), scc.SupplementalGroups.Type,
			[]string{string(securityapi.SupplementalGroupsStrategyMustRunAs), string(securityapi.SupplementalGroupsStrategyRunAsAny)}))
	}
	allErrs = append(allErrs, validateIDRanges(scc.SupplementalGroups.Ranges, field.NewPath("supplementalGroups"))...)

	// validate capabilities
	allErrs = append(allErrs, validateSCCCapsAgainstDrops(scc.RequiredDropCapabilities, scc.DefaultAddCapabilities, field.NewPath("defaultAddCapabilities"))...)
	allErrs = append(allErrs, validateSCCCapsAgainstDrops(scc.RequiredDropCapabilities, scc.AllowedCapabilities, field.NewPath("allowedCapabilities"))...)

	if hasCap(securityapi.AllowAllCapabilities, scc.AllowedCapabilities) && len(scc.RequiredDropCapabilities) > 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("requiredDropCapabilities"), scc.RequiredDropCapabilities,
			"required capabilities must be empty when all capabilities are allowed by a wildcard"))
	}

	allowsFlexVolumes := false
	hasNoneVolume := false

	if len(scc.Volumes) > 0 {
		for _, fsType := range scc.Volumes {
			if fsType == securityapi.FSTypeNone {
				hasNoneVolume = true

			} else if fsType == securityapi.FSTypeFlexVolume || fsType == securityapi.FSTypeAll {
				allowsFlexVolumes = true
			}
		}
	}

	if hasNoneVolume && len(scc.Volumes) > 1 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("volumes"), scc.Volumes,
			"if 'none' is specified, no other values are allowed"))
	}

	if len(scc.AllowedFlexVolumes) > 0 {
		if allowsFlexVolumes {
			for idx, allowedFlexVolume := range scc.AllowedFlexVolumes {
				if len(allowedFlexVolume.Driver) == 0 {
					allErrs = append(allErrs, field.Required(field.NewPath("allowedFlexVolumes").Index(idx).Child("driver"),
						"must specify a driver"))
				}
			}
		} else {
			allErrs = append(allErrs, field.Invalid(field.NewPath("allowedFlexVolumes"), scc.AllowedFlexVolumes,
				"volumes does not include 'flexVolume' or '*', so no flex volumes are allowed"))
		}
	}

	return allErrs
}

// validateSCCCapsAgainstDrops ensures an allowed cap is not listed in the required drops.
func validateSCCCapsAgainstDrops(requiredDrops []kapi.Capability, capsToCheck []kapi.Capability, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if requiredDrops == nil {
		return allErrs
	}
	for _, cap := range capsToCheck {
		if hasCap(cap, requiredDrops) {
			allErrs = append(allErrs, field.Invalid(fldPath, cap,
				fmt.Sprintf("capability is listed in %s and requiredDropCapabilities", fldPath.String())))
		}
	}
	return allErrs
}

// hasCap checks for needle in haystack.
func hasCap(needle kapi.Capability, haystack []kapi.Capability) bool {
	for _, c := range haystack {
		if needle == c {
			return true
		}
	}
	return false
}

// validateIDRanges ensures the range is valid.
func validateIDRanges(rng []securityapi.IDRange, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, r := range rng {
		// if 0 <= Min <= Max then we do not need to validate max.  It is always greater than or
		// equal to 0 and Min.
		minPath := fldPath.Child("ranges").Index(i).Child("min")
		maxPath := fldPath.Child("ranges").Index(i).Child("max")

		if r.Min < 0 {
			allErrs = append(allErrs, field.Invalid(minPath, r.Min, "min cannot be negative"))
		}
		if r.Max < 0 {
			allErrs = append(allErrs, field.Invalid(maxPath, r.Max, "max cannot be negative"))
		}
		if r.Min > r.Max {
			allErrs = append(allErrs, field.Invalid(minPath, r, "min cannot be greater than max"))
		}
	}

	return allErrs
}

func ValidateSecurityContextConstraintsUpdate(newScc, oldScc *securityapi.SecurityContextConstraints) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&newScc.ObjectMeta, &oldScc.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateSecurityContextConstraints(newScc)...)
	return allErrs
}

// ValidatePodSecurityPolicySubjectReview validates PodSecurityPolicySubjectReview.
func ValidatePodSecurityPolicySubjectReview(podSecurityPolicySubjectReview *securityapi.PodSecurityPolicySubjectReview) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validatePodSecurityPolicySubjectReviewSpec(&podSecurityPolicySubjectReview.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validatePodSecurityPolicySubjectReviewSpec(podSecurityPolicySubjectReviewSpec *securityapi.PodSecurityPolicySubjectReviewSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, kapivalidation.ValidatePodSpec(&podSecurityPolicySubjectReviewSpec.Template.Spec, fldPath.Child("template", "spec"))...)
	return allErrs
}

// ValidatePodSecurityPolicySelfSubjectReview validates PodSecurityPolicySelfSubjectReview.
func ValidatePodSecurityPolicySelfSubjectReview(podSecurityPolicySelfSubjectReview *securityapi.PodSecurityPolicySelfSubjectReview) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validatePodSecurityPolicySelfSubjectReviewSpec(&podSecurityPolicySelfSubjectReview.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validatePodSecurityPolicySelfSubjectReviewSpec(podSecurityPolicySelfSubjectReviewSpec *securityapi.PodSecurityPolicySelfSubjectReviewSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, kapivalidation.ValidatePodSpec(&podSecurityPolicySelfSubjectReviewSpec.Template.Spec, fldPath.Child("template", "spec"))...)
	return allErrs
}

// ValidatePodSecurityPolicyReview validates PodSecurityPolicyReview.
func ValidatePodSecurityPolicyReview(podSecurityPolicyReview *securityapi.PodSecurityPolicyReview) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validatePodSecurityPolicyReviewSpec(&podSecurityPolicyReview.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validatePodSecurityPolicyReviewSpec(podSecurityPolicyReviewSpec *securityapi.PodSecurityPolicyReviewSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, kapivalidation.ValidatePodSpec(&podSecurityPolicyReviewSpec.Template.Spec, fldPath.Child("template", "spec"))...)
	allErrs = append(allErrs, validateServiceAccountNames(podSecurityPolicyReviewSpec.ServiceAccountNames, fldPath.Child("serviceAccountNames"))...)
	return allErrs
}

func validateServiceAccountNames(serviceAccountNames []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, sa := range serviceAccountNames {
		idxPath := fldPath.Index(i)
		switch {
		case len(sa) == 0:
			allErrs = append(allErrs, field.Invalid(idxPath, sa, ""))
		case len(sa) > 0:
			if reasons := kapivalidation.ValidateServiceAccountName(sa, false); len(reasons) != 0 {
				allErrs = append(allErrs, field.Invalid(idxPath, sa, strings.Join(reasons, ", ")))
			}
		}
	}
	return allErrs
}
