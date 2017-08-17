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
	"github.com/kubernetes-incubator/service-catalog/pkg/controller"
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
	allErrs = append(allErrs, validateInstanceStatus(&instance.Status, field.NewPath("Status"), create)...)
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

	if spec.ParametersFrom != nil {
		for _, paramsFrom := range spec.ParametersFrom {
			if paramsFrom.SecretKeyRef != nil {
				if paramsFrom.SecretKeyRef.Name == "" {
					allErrs = append(allErrs, field.Required(fldPath.Child("parametersFrom.secretKeyRef.name"), "name is required"))
				}
				if paramsFrom.SecretKeyRef.Key == "" {
					allErrs = append(allErrs, field.Required(fldPath.Child("parametersFrom.secretKeyRef.key"), "key is required"))
				}
			} else {
				allErrs = append(allErrs, field.Required(fldPath.Child("parametersFrom"), "source must not be empty if present"))
			}
		}
	}
	if spec.Parameters != nil {
		if len(spec.Parameters.Raw) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("parameters"), "inline parameters must not be empty if present"))
		}
		if _, err := controller.UnmarshalRawParameters(spec.Parameters.Raw); err != nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("parameters"), "invalid inline parameters"))
		}
	}

	return allErrs
}

func validateInstanceStatus(spec *sc.InstanceStatus, fldPath *field.Path, create bool) field.ErrorList {
	errors := field.ErrorList{}
	// TODO(vaikas): Implement more comprehensive status validation.
	// https://github.com/kubernetes-incubator/service-catalog/issues/882

	// Do not allow the instance to be ready if an async operation is ongoing
	// ongoing
	if spec.AsyncOpInProgress {
		for _, c := range spec.Conditions {
			if c.Type == sc.InstanceConditionReady && c.Status == sc.ConditionTrue {
				errors = append(errors, field.Forbidden(fldPath.Child("Conditions"), "Can not set InstanceConditionReady to true when an async operation is in progress"))
			}
		}
	}

	return errors
}

// internalValidateInstanceUpdateAllowed ensures there is not an asynchronous
// operation ongoing with the instance before allowing an update to go through.
func internalValidateInstanceUpdateAllowed(new *sc.Instance, old *sc.Instance) field.ErrorList {
	errors := field.ErrorList{}
	if old.Status.AsyncOpInProgress {
		errors = append(errors, field.Forbidden(field.NewPath("Spec"), "Another operation for this service instance is in progress"))
	}
	return errors
}

// ValidateInstanceUpdate validates a change to the Instance's spec.
func ValidateInstanceUpdate(new *sc.Instance, old *sc.Instance) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, internalValidateInstanceUpdateAllowed(new, old)...)
	allErrs = append(allErrs, internalValidateInstance(new, false)...)
	return allErrs
}

func internalValidateInstanceStatusUpdateAllowed(new *sc.Instance, old *sc.Instance) field.ErrorList {
	errors := field.ErrorList{}
	// TODO(vaikas): Are there any cases where we do not allow updates to
	// Status during Async updates in progress?
	return errors
}

// ValidateInstanceStatusUpdate checks that when changing from an older
// instance to a newer instance is okay. This only checks the instance.Status field.
func ValidateInstanceStatusUpdate(new *sc.Instance, old *sc.Instance) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, internalValidateInstanceStatusUpdateAllowed(new, old)...)
	allErrs = append(allErrs, internalValidateInstance(new, false)...)
	return allErrs
}
