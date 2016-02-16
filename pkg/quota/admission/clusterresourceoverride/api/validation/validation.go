package validation

import (
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
)

func Validate(config *api.ClusterResourceOverrideConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	if config == nil {
		return allErrs
	}
	if config.LimitCPUToMemoryPercent == 0 && config.CPURequestToLimitPercent == 0 && config.MemoryRequestToLimitPercent == 0 {
		allErrs = append(allErrs, field.Forbidden(field.NewPath(api.PluginName), "plugin enabled but no percentages were specified"))
	}
	if config.LimitCPUToMemoryPercent < 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath(api.PluginName, "LimitCPUToMemoryPercent"), config.LimitCPUToMemoryPercent, "must be positive"))
	}
	if config.CPURequestToLimitPercent < 0 || config.CPURequestToLimitPercent > 100 {
		allErrs = append(allErrs, field.Invalid(field.NewPath(api.PluginName, "CPURequestToLimitPercent"), config.CPURequestToLimitPercent, "must be between 0 and 100"))
	}
	if config.MemoryRequestToLimitPercent < 0 || config.MemoryRequestToLimitPercent > 100 {
		allErrs = append(allErrs, field.Invalid(field.NewPath(api.PluginName, "MemoryRequestToLimitPercent"), config.MemoryRequestToLimitPercent, "must be between 0 and 100"))
	}
	return allErrs
}
