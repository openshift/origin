/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package validation

import (
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	sc "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
)

// validateBindingName is the validation function for Binding names.
var validateBindingName = apivalidation.NameIsDNSSubdomain

// ValidateBinding validates a Binding and returns a list of errors.
func ValidateBinding(binding *sc.Binding) field.ErrorList {
	return internalValidateBinding(binding, true)
}

func internalValidateBinding(binding *sc.Binding, create bool) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&binding.ObjectMeta, true, /*namespace*/
		validateBindingName,
		field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateBindingSpec(&binding.Spec, field.NewPath("Spec"), create)...)

	return allErrs
}

func validateBindingSpec(spec *sc.BindingSpec, fldPath *field.Path, create bool) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, msg := range validateInstanceName(spec.InstanceRef.Name, false /* prefix */) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("instanceRef", "name"), spec.InstanceRef.Name, msg))
	}

	for _, msg := range apivalidation.NameIsDNSSubdomain(spec.SecretName, false /* prefix */) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("secretName"), spec.SecretName, msg))
	}

	if spec.AlphaPodPresetTemplate != nil {
		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(&spec.AlphaPodPresetTemplate.Selector, fldPath.Child("alphaPodPresetTemplate", "selector"))...)

		for _, msg := range apivalidation.NameIsDNSSubdomain(spec.AlphaPodPresetTemplate.Name, false /* prefix */) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("alphaPodPresetTemplate", "name"), spec.AlphaPodPresetTemplate.Name, msg))
		}
	}

	return allErrs
}

// ValidateBindingUpdate checks that when changing from an older binding to a newer binding is okay.
func ValidateBindingUpdate(new *sc.Binding, old *sc.Binding) field.ErrorList {
	return internalValidateBinding(new, false)
}

// ValidateBindingStatusUpdate checks that when changing from an older binding to a newer binding is okay.
func ValidateBindingStatusUpdate(new *sc.Binding, old *sc.Binding) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateBindingUpdate(new, old)...)
	return allErrs
}
