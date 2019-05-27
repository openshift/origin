package validation

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/core/validation"

	configapi "github.com/openshift/openshift-apiserver/apis/config"
	buildvalidation "github.com/openshift/openshift-apiserver/pkg/build/apis/build/validation"
)

func ValidateBuildOverridesConfig(config *configapi.BuildOverridesConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, buildvalidation.ValidateImageLabels(config.ImageLabels, field.NewPath("imageLabels"))...)
	allErrs = append(allErrs, buildvalidation.ValidateNodeSelector(config.NodeSelector, field.NewPath("nodeSelector"))...)
	allErrs = append(allErrs, validation.ValidateAnnotations(config.Annotations, field.NewPath("annotations"))...)

	return allErrs
}
