package validation

import (
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/build/admission/defaults/api"
	buildvalidation "github.com/openshift/origin/pkg/build/api/validation"
)

// ValidateBuild tests required fields for a Build.
func ValidateBuildDefaultsConfig(config *api.BuildDefaultsConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateURL(config.GitHTTPProxy, field.NewPath("gitHTTPProxy"))...)
	allErrs = append(allErrs, validateURL(config.GitHTTPSProxy, field.NewPath("gitHTTPSProxy"))...)
	allErrs = append(allErrs, buildvalidation.ValidateStrategyEnv(config.Env, field.NewPath("env"))...)
	return allErrs
}

//
func validateURL(u string, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if !buildvalidation.IsValidURL(u) {
		allErrs = append(allErrs, field.Invalid(path, u, "invalid URL"))
	}
	return allErrs
}
