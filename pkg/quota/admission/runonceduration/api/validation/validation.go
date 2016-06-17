package validation

import (
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/quota/admission/runonceduration/api"
)

// ValidateRunOnceDurationConfig validates the RunOnceDuration plugin configuration
func ValidateRunOnceDurationConfig(config *api.RunOnceDurationConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	if config == nil || config.ActiveDeadlineSecondsLimit == nil {
		return allErrs
	}
	if *config.ActiveDeadlineSecondsLimit <= 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("activeDeadlineSecondsOverride"), config.ActiveDeadlineSecondsLimit, "must be greater than 0"))
	}
	return allErrs
}
