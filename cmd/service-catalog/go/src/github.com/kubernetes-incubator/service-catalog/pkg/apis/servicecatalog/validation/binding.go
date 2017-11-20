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
	"github.com/ghodss/yaml"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	sc "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
)

// validateServiceBindingName is the validation function for ServiceBinding names.
var validateServiceBindingName = apivalidation.NameIsDNSSubdomain

var validServiceBindingOperations = map[sc.ServiceBindingOperation]bool{
	sc.ServiceBindingOperation(""):   true,
	sc.ServiceBindingOperationBind:   true,
	sc.ServiceBindingOperationUnbind: true,
}

var validServiceBindingOperationValues = func() []string {
	validValues := make([]string, len(validServiceBindingOperations))
	i := 0
	for operation := range validServiceBindingOperations {
		validValues[i] = string(operation)
		i++
	}
	return validValues
}()

var validServiceBindingUnbindStatuses = map[sc.ServiceBindingUnbindStatus]bool{
	sc.ServiceBindingUnbindStatusNotRequired: true,
	sc.ServiceBindingUnbindStatusRequired:    true,
	sc.ServiceBindingUnbindStatusSucceeded:   true,
	sc.ServiceBindingUnbindStatusFailed:      true,
}

var validServiceBindingUnbindStatusValues = func() []string {
	validValues := make([]string, len(validServiceBindingUnbindStatuses))
	i := 0
	for operation := range validServiceBindingUnbindStatuses {
		validValues[i] = string(operation)
		i++
	}
	return validValues
}()

// ValidateServiceBinding validates a ServiceBinding and returns a list of errors.
func ValidateServiceBinding(binding *sc.ServiceBinding) field.ErrorList {
	return internalValidateServiceBinding(binding, true)
}

func internalValidateServiceBinding(binding *sc.ServiceBinding, create bool) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&binding.ObjectMeta, true, /*namespace*/
		validateServiceBindingName,
		field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateServiceBindingSpec(&binding.Spec, field.NewPath("spec"), create)...)
	allErrs = append(allErrs, validateServiceBindingStatus(&binding.Status, field.NewPath("status"), create)...)
	if create {
		allErrs = append(allErrs, validateServiceBindingCreate(binding)...)
	} else {
		allErrs = append(allErrs, validateServiceBindingUpdate(binding)...)
	}
	return allErrs
}

func validateServiceBindingSpec(spec *sc.ServiceBindingSpec, fldPath *field.Path, create bool) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, msg := range validateServiceInstanceName(spec.ServiceInstanceRef.Name, false /* prefix */) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("instanceRef", "name"), spec.ServiceInstanceRef.Name, msg))
	}

	for _, msg := range apivalidation.NameIsDNSSubdomain(spec.SecretName, false /* prefix */) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("secretName"), spec.SecretName, msg))
	}

	return allErrs
}

func validateServiceBindingStatus(status *sc.ServiceBindingStatus, fldPath *field.Path, create bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if create {
		if status.CurrentOperation != "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("currentOperation"), status.CurrentOperation, "currentOperation must be empty on create"))
		}
	} else {
		if !validServiceBindingOperations[status.CurrentOperation] {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("currentOperation"), status.CurrentOperation, validServiceBindingOperationValues))
		}
	}

	if status.CurrentOperation == "" {
		if status.OperationStartTime != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("operationStartTime"), "operationStartTime must not be present when currentOperation is not present"))
		}
		if status.AsyncOpInProgress {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("asyncOpInProgress"), "asyncOpInProgress cannot be true when there is no currentOperation"))
		}
	} else {
		if status.OperationStartTime == nil && !status.OrphanMitigationInProgress {
			allErrs = append(allErrs, field.Required(fldPath.Child("operationStartTime"), "operationStartTime is required when currentOperation is present and no orphan mitigation in progress"))
		}
		// Do not allow the binding to be ready if there is an on-going operation
		for i, c := range status.Conditions {
			if c.Type == sc.ServiceBindingConditionReady && c.Status == sc.ConditionTrue {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("conditions").Index(i), "Can not set ServiceBindingConditionReady to true when there is an operation in progress"))
			}
		}
	}

	if status.CurrentOperation == sc.ServiceBindingOperationBind {
		if status.InProgressProperties == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("inProgressProperties"), `inProgressProperties is required when currentOperation is "Bind"`))
		}
	} else {
		if status.InProgressProperties != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("inProgressProperties"), `inProgressProperties must not be present when currentOperation is not "Bind"`))
		}
	}

	if status.InProgressProperties != nil {
		allErrs = append(allErrs, validateServiceBindingPropertiesState(status.InProgressProperties, fldPath.Child("inProgressProperties"), create)...)
	}

	if status.ExternalProperties != nil {
		allErrs = append(allErrs, validateServiceBindingPropertiesState(status.ExternalProperties, fldPath.Child("externalProperties"), create)...)
	}

	if create {
		if status.UnbindStatus != sc.ServiceBindingUnbindStatusNotRequired {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("unbindStatus"), status.UnbindStatus, `unbindStatus must be "NotRequired" on create`))
		}
	} else {
		if !validServiceBindingUnbindStatuses[status.UnbindStatus] {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("unbindStatus"), status.UnbindStatus, validServiceBindingUnbindStatusValues))
		}
	}

	return allErrs
}

func validateServiceBindingPropertiesState(propertiesState *sc.ServiceBindingPropertiesState, fldPath *field.Path, create bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if propertiesState.Parameters == nil {
		if propertiesState.ParametersChecksum != "" {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("parametersChecksum"), "parametersChecksum must be empty when there are no parameters"))
		}
	} else {
		if len(propertiesState.Parameters.Raw) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("parameters").Child("raw"), "raw must not be empty"))
		} else {
			unmarshalled := make(map[string]interface{})
			if err := yaml.Unmarshal(propertiesState.Parameters.Raw, &unmarshalled); err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("parameters").Child("raw"), propertiesState.Parameters.Raw, "raw must be valid yaml"))
			}
		}
		if propertiesState.ParametersChecksum == "" {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("parametersChecksum"), "parametersChecksum must not be empty when there are parameters"))
		}
	}

	if propertiesState.ParametersChecksum != "" {
		if len(propertiesState.ParametersChecksum) != 64 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("parametersChecksum"), propertiesState.ParametersChecksum, "parametersChecksum must be exactly 64 digits"))
		}
		if !stringIsHexadecimal(propertiesState.ParametersChecksum) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("parametersChecksum"), propertiesState.ParametersChecksum, "parametersChecksum must be a hexadecimal number"))
		}
	}

	return allErrs
}

func validateServiceBindingCreate(binding *sc.ServiceBinding) field.ErrorList {
	allErrs := field.ErrorList{}
	if binding.Status.ReconciledGeneration >= binding.Generation {
		allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("reconciledGeneration"), binding.Status.ReconciledGeneration, "reconciledGeneration must be less than generation on create"))
	}
	return allErrs
}

func validateServiceBindingUpdate(binding *sc.ServiceBinding) field.ErrorList {
	allErrs := field.ErrorList{}
	if binding.Status.ReconciledGeneration == binding.Generation {
		if binding.Status.CurrentOperation != "" {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("status").Child("currentOperation"), "currentOperation must not be present when reconciledGeneration and generation are equal"))
		}
	} else if binding.Status.ReconciledGeneration > binding.Generation {
		allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("reconciledGeneration"), binding.Status.ReconciledGeneration, "reconciledGeneration must not be greater than generation"))
	}
	return allErrs
}

// internalValidateServiceBindingUpdateAllowed ensures there is not a
// pending update on-going with the spec of the binding before allowing an update
// to the spec to go through.
func internalValidateServiceBindingUpdateAllowed(new *sc.ServiceBinding, old *sc.ServiceBinding) field.ErrorList {
	errors := field.ErrorList{}
	if old.Generation != new.Generation && old.Status.ReconciledGeneration != old.Generation {
		errors = append(errors, field.Forbidden(field.NewPath("spec"), "another change to the spec is in progress"))
	}
	return errors
}

// ValidateServiceBindingUpdate checks that when changing from an older binding to a newer binding is okay.
func ValidateServiceBindingUpdate(new *sc.ServiceBinding, old *sc.ServiceBinding) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, internalValidateServiceBindingUpdateAllowed(new, old)...)
	allErrs = append(allErrs, internalValidateServiceBinding(new, false)...)
	return allErrs
}

// ValidateServiceBindingStatusUpdate checks that when changing from an older binding to a newer binding is okay.
func ValidateServiceBindingStatusUpdate(new *sc.ServiceBinding, old *sc.ServiceBinding) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, internalValidateServiceBinding(new, false)...)
	return allErrs
}
