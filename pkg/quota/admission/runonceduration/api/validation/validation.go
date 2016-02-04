package validation

import (
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/quota/admission/runonceduration/api"
)

// ValidateRunOnceDurationConfig validates the RunOnceDuration plugin configuration
func ValidateRunOnceDurationConfig(config *api.RunOnceDurationConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	if config == nil || config.ActiveDeadlineSecondsOverride == nil {
		return allErrs
	}
	if *config.ActiveDeadlineSecondsOverride <= 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("activeDeadlineSecondsOverride"), config.ActiveDeadlineSecondsOverride, "must be greater than 0"))
	}
	return allErrs
}
