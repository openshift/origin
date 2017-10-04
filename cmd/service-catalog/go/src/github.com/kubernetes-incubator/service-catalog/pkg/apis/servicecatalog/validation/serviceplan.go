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
	"regexp"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	sc "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
)

const servicePlanNameFmt string = `[-a-z0-9]+`
const servicePlanNameMaxLength int = 63

var servicePlanNameRegexp = regexp.MustCompile("^" + servicePlanNameFmt + "$")

// validateServicePlanName is the validation function for ServicePlan names.
func validateServicePlanName(value string, prefix bool) []string {
	var errs []string
	if len(value) > servicePlanNameMaxLength {
		errs = append(errs, utilvalidation.MaxLenError(servicePlanNameMaxLength))
	}
	if !servicePlanNameRegexp.MatchString(value) {
		errs = append(errs, utilvalidation.RegexError(servicePlanNameFmt, "plan-name-40d-0983-1b89"))
	}

	return errs
}

// ValidateServicePlan validates a ServicePlan and returns a list of errors.
func ValidateServicePlan(serviceplan *sc.ServicePlan) field.ErrorList {
	return internalValidateServicePlan(serviceplan)
}

func internalValidateServicePlan(serviceplan *sc.ServicePlan) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs,
		apivalidation.ValidateObjectMeta(
			&serviceplan.ObjectMeta,
			false, /* namespace required */
			validateServicePlanName,
			field.NewPath("metadata"))...)

	allErrs = append(allErrs, validateServicePlanSpec(&serviceplan.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateServicePlanSpec(spec *sc.ServicePlanSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if "" == spec.ServiceBrokerName {
		allErrs = append(allErrs, field.Required(fldPath.Child("serviceBrokerName"), "serviceBrokerName is required"))
	}

	if "" == spec.ExternalID {
		allErrs = append(allErrs, field.Required(fldPath.Child("externalID"), "externalID is required"))
	}

	if "" == spec.Description {
		allErrs = append(allErrs, field.Required(fldPath.Child("description"), "description is required"))
	}

	if "" == spec.ServiceClassRef.Name {
		allErrs = append(allErrs, field.Required(fldPath.Child("serviceClassRef"), "an owning serviceclass is required"))
	}

	for _, msg := range validateServicePlanName(spec.ExternalName, false /* prefix */) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("externalName"), spec.ExternalName, msg))
	}

	for _, msg := range validateExternalID(spec.ExternalID) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("externalID"), spec.ExternalID, msg))
	}

	for _, msg := range validateServiceClassName(spec.ServiceClassRef.Name, false /* prefix */) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("serviceClassRef", "name"), spec.ServiceClassRef.Name, msg))
	}

	return allErrs
}

// ValidateServicePlanUpdate checks that when changing from an older
// ServicePlan to a newer ServicePlan is okay.
func ValidateServicePlanUpdate(new *sc.ServicePlan, old *sc.ServicePlan) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, internalValidateServicePlan(new)...)
	if new.Spec.ExternalID != old.Spec.ExternalID {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("externalID"), new.Spec.ExternalID, "externalID cannot change when updating a ServicePlan"))
	}
	return allErrs
}
