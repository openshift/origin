package validation

import (
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
)

func Validate(config *api.ImagePolicyConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	if config == nil {
		return allErrs
	}
	// if config.MemoryRequestToLimitPercent < 0 || config.MemoryRequestToLimitPercent > 100 {
	// 	allErrs = append(allErrs, field.Invalid(field.NewPath(api.PluginName, "MemoryRequestToLimitPercent"), config.MemoryRequestToLimitPercent, "must be between 0 and 100"))
	// }
	return allErrs
}
