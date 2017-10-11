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
	"github.com/kubernetes-incubator/service-catalog/pkg/controller"
)

// validateServiceInstanceName is the validation function for Instance names.
var validateServiceInstanceName = apivalidation.NameIsDNSSubdomain

var validServiceInstanceOperations = map[sc.ServiceInstanceOperation]bool{
	sc.ServiceInstanceOperation(""):        true,
	sc.ServiceInstanceOperationProvision:   true,
	sc.ServiceInstanceOperationUpdate:      true,
	sc.ServiceInstanceOperationDeprovision: true,
}

var validServiceInstanceOperationValues = func() []string {
	validValues := make([]string, len(validServiceInstanceOperations))
	i := 0
	for operation := range validServiceInstanceOperations {
		validValues[i] = string(operation)
		i++
	}
	return validValues
}()

// ValidateServiceInstance validates an Instance and returns a list of errors.
func ValidateServiceInstance(instance *sc.ServiceInstance) field.ErrorList {
	return internalValidateServiceInstance(instance, true)
}

func internalValidateServiceInstance(instance *sc.ServiceInstance, create bool) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&instance.ObjectMeta, true, /*namespace*/
		validateServiceInstanceName,
		field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateServiceInstanceSpec(&instance.Spec, field.NewPath("spec"), create)...)
	allErrs = append(allErrs, validateServiceInstanceStatus(&instance.Status, field.NewPath("status"), create)...)
	if create {
		allErrs = append(allErrs, validateServiceInstanceCreate(instance)...)
	} else {
		allErrs = append(allErrs, validateServiceInstanceUpdate(instance)...)
	}
	return allErrs
}

func validateServiceInstanceSpec(spec *sc.ServiceInstanceSpec, fldPath *field.Path, create bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if "" == spec.ExternalClusterServiceClassName {
		allErrs = append(allErrs, field.Required(fldPath.Child("externalClusterServiceClassName"), "externalClusterServiceClassName is required"))
	}
	for _, msg := range validateServiceClassName(spec.ExternalClusterServiceClassName, false /* prefix */) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("externalClusterServiceClassName"), spec.ExternalClusterServiceClassName, msg))
	}

	if "" == spec.ExternalClusterServicePlanName {
		allErrs = append(allErrs, field.Required(fldPath.Child("externalClusterServicePlanName"), "externalClusterServicePlanName is required"))
	}
	for _, msg := range validateServicePlanName(spec.ExternalClusterServicePlanName, false /* prefix */) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("externalClusterServicePlanName"), spec.ExternalClusterServicePlanName, msg))
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

	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(spec.UpdateRequests, fldPath.Child("updateRequests"))...)

	return allErrs
}

func validateServiceInstanceStatus(status *sc.ServiceInstanceStatus, fldPath *field.Path, create bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if create {
		if status.CurrentOperation != "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("currentOperation"), status.CurrentOperation, "currentOperation must be empty on create"))
		}
	} else {
		if !validServiceInstanceOperations[status.CurrentOperation] {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("currentOperation"), status.CurrentOperation, validServiceInstanceOperationValues))
		}
	}

	if status.CurrentOperation == "" {
		if status.OperationStartTime != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("operationStartTime"), "operationStartTime must not be present when currentOperation is not present"))
		}
		if status.AsyncOpInProgress {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("asyncOpInProgress"), "asyncOpInProgress cannot be true when there is no currentOperation"))
		}
		if status.LastOperation != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("lastOperation"), "lastOperation cannot be true when currentOperation is not present"))
		}
	} else {
		if status.OperationStartTime == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("operationStartTime"), "operationStartTime is required when currentOperation is present"))
		}
		// Do not allow the instance to be ready if there is an on-going operation
		for i, c := range status.Conditions {
			if c.Type == sc.ServiceInstanceConditionReady && c.Status == sc.ConditionTrue {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("conditions").Index(i), "Can not set ServiceInstanceConditionReady to true when there is an operation in progress"))
			}
		}
	}

	switch status.CurrentOperation {
	case sc.ServiceInstanceOperationProvision, sc.ServiceInstanceOperationUpdate:
		if status.InProgressProperties == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("inProgressProperties"), `inProgressProperties is required when currentOperation is "Provision" or "Update"`))
		}
	default:
		if status.InProgressProperties != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("inProgressProperties"), `inProgressProperties must not be present when currentOperation is neither "Provision" nor "Update"`))
		}
	}

	if status.InProgressProperties != nil {
		allErrs = append(allErrs, validateServiceInstancePropertiesState(status.InProgressProperties, fldPath.Child("inProgressProperties"), create)...)
	}

	if status.ExternalProperties != nil {
		allErrs = append(allErrs, validateServiceInstancePropertiesState(status.ExternalProperties, fldPath.Child("externalProperties"), create)...)
	}

	return allErrs
}

func validateServiceInstancePropertiesState(propertiesState *sc.ServiceInstancePropertiesState, fldPath *field.Path, create bool) field.ErrorList {
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

func validateServiceInstanceCreate(instance *sc.ServiceInstance) field.ErrorList {
	allErrs := field.ErrorList{}
	if instance.Status.ReconciledGeneration >= instance.Generation {
		allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("reconciledGeneration"), instance.Status.ReconciledGeneration, "reconciledGeneration must be less than generation on create"))
	}
	if instance.Spec.ClusterServiceClassRef != nil {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec").Child("clusterServiceClassRef"), "clusterServiceClassRef must not be present on create"))
	}
	if instance.Spec.ClusterServicePlanRef != nil {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec").Child("clusterServicePlanRef"), "clusterServicePlanRef must not be present on create"))
	}
	return allErrs
}

func validateServiceInstanceUpdate(instance *sc.ServiceInstance) field.ErrorList {
	allErrs := field.ErrorList{}
	if instance.Status.ReconciledGeneration == instance.Generation {
		if instance.Status.CurrentOperation != "" {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("status").Child("currentOperation"), "currentOperation must not be present when reconciledGeneration and generation are equal"))
		}
	} else if instance.Status.ReconciledGeneration > instance.Generation {
		allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("reconciledGeneration"), instance.Status.ReconciledGeneration, "reconciledGeneration must not be greater than generation"))
	}
	if instance.Status.CurrentOperation != "" {
		if instance.Spec.ClusterServiceClassRef == nil {
			allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("clusterServceClassRef"), "serviceClassRef is required when currentOperation is present"))
		}
		if instance.Spec.ClusterServicePlanRef == nil {
			allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("clusterServicePlanRef"), "servicePlanRef is required when currentOperation is present"))
		}
	}
	return allErrs
}

// internalValidateServiceInstanceUpdateAllowed ensures there is not a
// pending update on-going with the spec of the instance before allowing an update
// to the spec to go through.
func internalValidateServiceInstanceUpdateAllowed(new *sc.ServiceInstance, old *sc.ServiceInstance) field.ErrorList {
	errors := field.ErrorList{}
	if old.Generation != new.Generation && old.Status.ReconciledGeneration != old.Generation {
		errors = append(errors, field.Forbidden(field.NewPath("spec"), "Another update for this service instance is in progress"))
	}
	if old.Spec.ExternalClusterServicePlanName != new.Spec.ExternalClusterServicePlanName && new.Spec.ClusterServicePlanRef != nil {
		errors = append(errors, field.Forbidden(field.NewPath("spec").Child("clusterServicePlanRef"), "clusterServicePlanRef must not be present when externalServicePlanName is being changed"))
	}
	return errors
}

// ValidateServiceInstanceUpdate validates a change to the Instance's spec.
func ValidateServiceInstanceUpdate(new *sc.ServiceInstance, old *sc.ServiceInstance) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, internalValidateServiceInstanceUpdateAllowed(new, old)...)
	allErrs = append(allErrs, internalValidateServiceInstance(new, false)...)

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Spec.ExternalClusterServiceClassName, old.Spec.ExternalClusterServiceClassName, field.NewPath("spec").Child("externalClusterServiceClassName"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Spec.ExternalID, old.Spec.ExternalID, field.NewPath("spec").Child("externalID"))...)

	if new.Spec.UpdateRequests < old.Spec.UpdateRequests {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("updateRequests"), new.Spec.UpdateRequests, "new updateRequests value must not be less than the old one"))
	}

	return allErrs
}

func internalValidateServiceInstanceStatusUpdateAllowed(new *sc.ServiceInstance, old *sc.ServiceInstance) field.ErrorList {
	errors := field.ErrorList{}
	// TODO(vaikas): Are there any cases where we do not allow updates to
	// Status during Async updates in progress?
	return errors
}

func internalValidateServiceInstanceReferencesUpdateAllowed(new *sc.ServiceInstance, old *sc.ServiceInstance) field.ErrorList {
	allErrs := field.ErrorList{}
	if new.Status.CurrentOperation != "" {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("status").Child("currentOperation"), "cannot update references when currentOperation is present"))
	}
	if new.Spec.ClusterServiceClassRef == nil {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("clusterServiceClassRef"), "clusterServiceClassRef is required when updating references"))
	}
	if new.Spec.ClusterServicePlanRef == nil {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("clusterServicePlanRef"), "clusterServicePlanRef is required when updating references"))
	}
	if old.Spec.ClusterServiceClassRef != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Spec.ClusterServiceClassRef, old.Spec.ClusterServiceClassRef, field.NewPath("spec").Child("clusterServiceClassRef"))...)
	}
	if old.Spec.ClusterServicePlanRef != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Spec.ClusterServicePlanRef, old.Spec.ClusterServicePlanRef, field.NewPath("spec").Child("clusterServicePlanRef"))...)
	}
	return allErrs
}

// ValidateServiceInstanceStatusUpdate checks that when changing from an older
// instance to a newer instance is okay. This only checks the instance.Status field.
func ValidateServiceInstanceStatusUpdate(new *sc.ServiceInstance, old *sc.ServiceInstance) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, internalValidateServiceInstanceStatusUpdateAllowed(new, old)...)
	allErrs = append(allErrs, internalValidateServiceInstance(new, false)...)
	return allErrs
}

// ValidateServiceInstanceReferencesUpdate checks that when changing from an older
// instance to a newer instance is okay.
func ValidateServiceInstanceReferencesUpdate(new *sc.ServiceInstance, old *sc.ServiceInstance) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, internalValidateServiceInstanceReferencesUpdateAllowed(new, old)...)
	allErrs = append(allErrs, internalValidateServiceInstance(new, false)...)
	return allErrs
}
