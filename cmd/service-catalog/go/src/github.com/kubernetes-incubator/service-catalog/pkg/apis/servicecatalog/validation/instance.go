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

var validServiceInstanceDeprovisionStatuses = map[sc.ServiceInstanceDeprovisionStatus]bool{
	sc.ServiceInstanceDeprovisionStatusNotRequired: true,
	sc.ServiceInstanceDeprovisionStatusRequired:    true,
	sc.ServiceInstanceDeprovisionStatusSucceeded:   true,
	sc.ServiceInstanceDeprovisionStatusFailed:      true,
}

var validServiceInstanceDeprovisionStatusValues = func() []string {
	validValues := make([]string, len(validServiceInstanceDeprovisionStatuses))
	i := 0
	for operation := range validServiceInstanceDeprovisionStatuses {
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

	allErrs = append(allErrs, validatePlanReference(&spec.PlanReference, fldPath)...)

	if spec.ParametersFrom != nil {
		allErrs = append(allErrs, validateParametersFromSource(spec.ParametersFrom, fldPath)...)
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
		if status.OperationStartTime == nil && !status.OrphanMitigationInProgress {
			allErrs = append(allErrs, field.Required(fldPath.Child("operationStartTime"), "operationStartTime is required when currentOperation is present and no orphan mitigation in progress"))
		}
		// Do not allow the instance to be ready if there is an on-going operation
		for i, c := range status.Conditions {
			if c.Type == sc.ServiceInstanceConditionReady && c.Status == sc.ConditionTrue {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("conditions").Index(i), "Can not set ServiceInstanceConditionReady to true when there is an operation in progress"))
			}
		}
	}

	switch status.CurrentOperation {
	case sc.ServiceInstanceOperationProvision, sc.ServiceInstanceOperationUpdate, sc.ServiceInstanceOperationDeprovision:
		if status.InProgressProperties == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("inProgressProperties"), `inProgressProperties is required when currentOperation is "Provision", "Update" or "Deprovision"`))
		}
	default:
		if status.InProgressProperties != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("inProgressProperties"), `inProgressProperties must not be present when currentOperation is not "Provision", "Update" or "Deprovision"`))
		}
	}

	if status.InProgressProperties != nil {
		allErrs = append(allErrs, validateServiceInstancePropertiesState(status.InProgressProperties, fldPath.Child("inProgressProperties"), create)...)
	}

	if status.ExternalProperties != nil {
		allErrs = append(allErrs, validateServiceInstancePropertiesState(status.ExternalProperties, fldPath.Child("externalProperties"), create)...)
	}

	if create {
		if status.DeprovisionStatus != sc.ServiceInstanceDeprovisionStatusNotRequired {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("deprovisionStatus"), status.DeprovisionStatus, `deprovisionStatus must be "NotRequired" on create`))
		}
	} else {
		if !validServiceInstanceDeprovisionStatuses[status.DeprovisionStatus] {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("deprovisionStatus"), status.DeprovisionStatus, validServiceInstanceDeprovisionStatusValues))
		}
	}

	return allErrs
}

func validateServiceInstancePropertiesState(propertiesState *sc.ServiceInstancePropertiesState, fldPath *field.Path, create bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if propertiesState.ClusterServicePlanExternalName == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("clusterServicePlanExternalName"), "clusterServicePlanExternalName is required"))
	}

	if propertiesState.ClusterServicePlanExternalID == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("clusterServicePlanExternalID"), "clusterServicePlanExternalID is required"))
	}

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
			allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("clusterServiceClassRef"), "serviceClassRef is required when currentOperation is present"))
		}
		if instance.Status.CurrentOperation != sc.ServiceInstanceOperationDeprovision {
			if instance.Spec.ClusterServicePlanRef == nil {
				allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("clusterServicePlanRef"), "servicePlanRef is required when currentOperation is present and not Deprovision"))
			}
		} else {
			if instance.Spec.ClusterServicePlanRef == nil &&
				(instance.Status.ExternalProperties == nil || instance.Status.ExternalProperties.ClusterServicePlanExternalID == "") {
				allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("currentOperation"), instance.Status.CurrentOperation, "spec.clusterServicePlanRef or status.externalProperties.clusterServicePlanExternalID is required when currentOperation is Deprovision"))
			}
		}
	}
	return allErrs
}

// internalValidateServiceInstanceUpdateAllowed ensures there is not a
// pending update on-going with the spec of the instance before allowing an update
// to the spec to go through.
func internalValidateServiceInstanceUpdateAllowed(new *sc.ServiceInstance, old *sc.ServiceInstance) field.ErrorList {
	errors := field.ErrorList{}
	if old.Generation != new.Generation && old.Status.CurrentOperation != "" {
		errors = append(errors, field.Forbidden(field.NewPath("spec"), "Another update for this service instance is in progress"))
	}
	if old.Spec.ClusterServicePlanExternalName != new.Spec.ClusterServicePlanExternalName && new.Spec.ClusterServicePlanRef != nil {
		errors = append(errors, field.Forbidden(field.NewPath("spec").Child("clusterServicePlanRef"), "clusterServicePlanRef must not be present when clusterServicePlanExternalName is being changed"))
	}
	return errors
}

// ValidateServiceInstanceUpdate validates a change to the Instance's spec.
func ValidateServiceInstanceUpdate(new *sc.ServiceInstance, old *sc.ServiceInstance) field.ErrorList {
	allErrs := field.ErrorList{}

	specFieldPath := field.NewPath("spec")

	allErrs = append(allErrs, validatePlanReferenceUpdate(&new.Spec.PlanReference, &old.Spec.PlanReference, specFieldPath)...)
	allErrs = append(allErrs, internalValidateServiceInstanceUpdateAllowed(new, old)...)
	allErrs = append(allErrs, internalValidateServiceInstance(new, false)...)

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Spec.ClusterServiceClassExternalName, old.Spec.ClusterServiceClassExternalName, specFieldPath.Child("clusterServiceClassExternalName"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Spec.ExternalID, old.Spec.ExternalID, specFieldPath.Child("externalID"))...)

	if new.Spec.UpdateRequests < old.Spec.UpdateRequests {
		allErrs = append(allErrs, field.Invalid(specFieldPath.Child("updateRequests"), new.Spec.UpdateRequests, "new updateRequests value must not be less than the old one"))
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

func validatePlanReference(p *sc.PlanReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Just to make reading of the conditionals in the code easier.
	externalClassSet := p.ClusterServiceClassExternalName != ""
	externalPlanSet := p.ClusterServicePlanExternalName != ""
	k8sClassSet := p.ClusterServiceClassName != ""
	k8sPlanSet := p.ClusterServicePlanName != ""

	// Can't specify both External and k8s name but must specify one.
	if externalClassSet == k8sClassSet {
		allErrs = append(allErrs, field.Required(fldPath.Child("clusterServiceClassExternalName"), "exactly one of clusterServiceClassExternalName or clusterServiceClassName required"))
		allErrs = append(allErrs, field.Required(fldPath.Child("clusterServiceClassName"), "exactly one of clusterServiceClassExternalName or clusterServiceClassName required"))
	}
	// Can't specify both External and k8s name but must specify one.
	if externalPlanSet == k8sPlanSet {
		allErrs = append(allErrs, field.Required(fldPath.Child("clusterServicePlanExternalName"), "exactly one of clusterServicePlanExternalName or clusterServicePlanName required"))
		allErrs = append(allErrs, field.Required(fldPath.Child("clusterServicePlanName"), "exactly one of clusterServicePlanExternalName or clusterServicePlanName required"))
	}

	if externalClassSet {
		for _, msg := range validateCommonServiceClassName(p.ClusterServiceClassExternalName, false /* prefix */) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("clusterServiceClassExternalName"), p.ClusterServiceClassExternalName, msg))
		}

		// If ClusterServiceClassExternalName given, must use ClusterServicePlanExternalName
		if !externalPlanSet {
			allErrs = append(allErrs, field.Required(fldPath.Child("clusterServicePlanExternalName"), "must specify clusterServicePlanExternalName with clusterServiceClassExternalName"))
		}

		for _, msg := range validateServicePlanName(p.ClusterServicePlanExternalName, false /* prefix */) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("clusterServicePlanExternalName"), p.ClusterServicePlanName, msg))
		}
	}
	if k8sClassSet {
		for _, msg := range validateCommonServiceClassName(p.ClusterServiceClassName, false /* prefix */) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("clusterServiceClassName"), p.ClusterServiceClassName, msg))
		}

		// If ClusterServiceClassName given, must use ClusterServicePlanName
		if !k8sPlanSet {
			allErrs = append(allErrs, field.Required(fldPath.Child("clusterServicePlanName"), "must specify clusterServicePlanName with clusterServiceClassName"))
		}
		for _, msg := range validateServicePlanName(p.ClusterServicePlanName, false /* prefix */) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("clusterServicePlanName"), p.ClusterServicePlanName, msg))
		}
	}
	return allErrs
}

func validatePlanReferenceUpdate(pOld *sc.PlanReference, pNew *sc.PlanReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validatePlanReference(pOld, fldPath)...)
	allErrs = append(allErrs, validatePlanReference(pNew, fldPath)...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(pNew.ClusterServiceClassExternalName, pOld.ClusterServiceClassExternalName, field.NewPath("spec").Child("clusterServiceClassExternalName"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(pNew.ClusterServiceClassName, pOld.ClusterServiceClassName, field.NewPath("spec").Child("clusterServiceClassName"))...)
	return allErrs
}
