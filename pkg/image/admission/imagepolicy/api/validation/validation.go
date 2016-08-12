package validation

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
)

func Validate(config *api.ImagePolicyConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	if config == nil {
		return allErrs
	}
	names := sets.NewString()
	for i, rule := range config.ExecutionRules {
		if names.Has(rule.Name) {
			allErrs = append(allErrs, field.Duplicate(field.NewPath(api.PluginName, "executionRules").Index(i).Child("name"), rule.Name))
		}
		names.Insert(rule.Name)
		for j, selector := range rule.MatchImageLabels {
			_, err := unversioned.LabelSelectorAsSelector(&selector)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath(api.PluginName, "executionRules").Index(i).Child("matchImageLabels").Index(j), nil, err.Error()))
			}
		}
	}
	return allErrs
}
