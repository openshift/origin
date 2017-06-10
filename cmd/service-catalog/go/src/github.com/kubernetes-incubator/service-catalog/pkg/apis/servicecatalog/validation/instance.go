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
	"k8s.io/apimachinery/pkg/util/validation/field"

	sc "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
)

// validateInstanceName is the validation function for Instance names.
var validateInstanceName = apivalidation.NameIsDNSSubdomain

// ValidateInstance validates an Instance and returns a list of errors.
func ValidateInstance(instance *sc.Instance) field.ErrorList {
	return internalValidateInstance(instance, true)
}

func internalValidateInstance(instance *sc.Instance, create bool) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&instance.ObjectMeta, true, /*namespace*/
		validateInstanceName,
		field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateInstanceSpec(&instance.Spec, field.NewPath("Spec"), create)...)
	return allErrs
}

func validateInstanceSpec(spec *sc.InstanceSpec, fldPath *field.Path, create bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if "" == spec.ServiceClassName {
		allErrs = append(allErrs, field.Required(fldPath.Child("serviceClassName"), "serviceClassName is required"))
	}

	for _, msg := range validateServiceClassName(spec.ServiceClassName, false /* prefix */) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("serviceClassName"), spec.ServiceClassName, msg))
	}

	if "" == spec.PlanName {
		allErrs = append(allErrs, field.Required(fldPath.Child("planName"), "planName is required"))
	}

	for _, msg := range validateServicePlanName(spec.PlanName) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("planName"), spec.PlanName, msg))
	}

	return allErrs
}

// ValidateInstanceUpdate validates a change to the Instance's spec.
func ValidateInstanceUpdate(new *sc.Instance, old *sc.Instance) field.ErrorList {
	return internalValidateInstance(new, false)
}

// ValidateInstanceStatusUpdate checks that when changing from an older
// instance to a newer instance is okay.
func ValidateInstanceStatusUpdate(new *sc.Instance, old *sc.Instance) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateInstanceUpdate(new, old)...)
	return allErrs
}
