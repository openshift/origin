package requestlimit

import (
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util/validation/field"
)

func ValidateProjectRequestLimitConfig(config *ProjectRequestLimitConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, projectLimit := range config.Limits {
		allErrs = append(allErrs, ValidateProjectLimitBySelector(projectLimit, field.NewPath("limits").Index(i))...)
	}
	return allErrs
}

func ValidateProjectLimitBySelector(limit ProjectLimitBySelector, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validation.ValidateLabels(limit.Selector, path.Child("selector"))...)
	if limit.MaxProjects != nil && *limit.MaxProjects < 0 {
		allErrs = append(allErrs, field.Invalid(path.Child("maxProjects"), *limit.MaxProjects, "cannot be a negative number"))
	}
	return allErrs
}
