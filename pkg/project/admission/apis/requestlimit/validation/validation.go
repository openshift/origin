package validation

import (
	unversionedvalidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/project/admission/apis/requestlimit"
)

func ValidateProjectRequestLimitConfig(config *requestlimit.ProjectRequestLimitConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, projectLimit := range config.Limits {
		allErrs = append(allErrs, ValidateProjectLimitBySelector(projectLimit, field.NewPath("limits").Index(i))...)
	}
	if config.MaxProjectsForSystemUsers != nil && *config.MaxProjectsForSystemUsers < 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("maxProjectsForSystemUsers"), *config.MaxProjectsForSystemUsers, "cannot be a negative number"))
	}
	if config.MaxProjectsForServiceAccounts != nil && *config.MaxProjectsForServiceAccounts < 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("maxProjectsForServiceAccounts"), *config.MaxProjectsForServiceAccounts, "cannot be a negative number"))
	}
	return allErrs
}

func ValidateProjectLimitBySelector(limit requestlimit.ProjectLimitBySelector, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, unversionedvalidation.ValidateLabels(limit.Selector, path.Child("selector"))...)
	if limit.MaxProjects != nil && *limit.MaxProjects < 0 {
		allErrs = append(allErrs, field.Invalid(path.Child("maxProjects"), *limit.MaxProjects, "cannot be a negative number"))
	}
	return allErrs
}
