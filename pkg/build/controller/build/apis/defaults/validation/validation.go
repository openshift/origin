package validation

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/core/validation"

	buildvalidation "github.com/openshift/origin/pkg/build/apis/build/validation"
	"github.com/openshift/origin/pkg/build/controller/build/apis/defaults"
	"github.com/openshift/origin/pkg/build/util"
)

// ValidateBuildDefaultsConfig tests required fields for a Build.
func ValidateBuildDefaultsConfig(config *defaults.BuildDefaultsConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateProxyURL(config.GitHTTPProxy, field.NewPath("gitHTTPProxy"))...)
	allErrs = append(allErrs, validateProxyURL(config.GitHTTPSProxy, field.NewPath("gitHTTPSProxy"))...)
	allErrs = append(allErrs, buildvalidation.ValidateStrategyEnv(config.Env, field.NewPath("env"))...)
	allErrs = append(allErrs, buildvalidation.ValidateImageLabels(config.ImageLabels, field.NewPath("imageLabels"))...)
	allErrs = append(allErrs, buildvalidation.ValidateNodeSelector(config.NodeSelector, field.NewPath("nodeSelector"))...)
	allErrs = append(allErrs, validation.ValidateAnnotations(config.Annotations, field.NewPath("annotations"))...)
	return allErrs
}

//
func validateProxyURL(u string, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if _, err := util.ParseProxyURL(u); err != nil {
		allErrs = append(allErrs, field.Invalid(path, u, err.Error()))
	}
	return allErrs
}
