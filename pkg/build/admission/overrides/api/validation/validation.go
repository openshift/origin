package validation

import (
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/build/admission/overrides/api"
	buildvalidation "github.com/openshift/origin/pkg/build/api/validation"
)

func ValidateBuildOverridesConfig(config *api.BuildOverridesConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, buildvalidation.ValidateImageLabels(config.ImageLabels, field.NewPath("imageLabels"))...)
	allErrs = append(allErrs, buildvalidation.ValidateNodeSelector(config.NodeSelector, field.NewPath("nodeSelector"))...)
	allErrs = append(allErrs, validation.ValidateAnnotations(config.Annotations, field.NewPath("annotations"))...)

	return allErrs
}
