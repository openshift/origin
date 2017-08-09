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
	"k8s.io/apimachinery/pkg/util/sets"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	sc "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
)

// validateServiceClassName is the validation function for ServiceClass names.
var validateServiceClassName = apivalidation.NameIsDNSSubdomain

const guidFmt string = "[a-zA-Z0-9]([-a-zA-Z0-9.]*[a-zA-Z0-9])?"
const guidMaxLength int = 63

// guidRegexp is a loosened validation for
// DNS1123 labels that allows uppercase characters.
var guidRegexp = regexp.MustCompile("^" + guidFmt + "$")

// validateExternalID is the validation function for External IDs that
// have been passed in. External IDs used to be OpenServiceBrokerAPI
// GUIDs, so we will retain that form until there is another provider
// that desires a different form.  In the case of the OSBAPI we
// generate GUIDs for Instances and Bindings, but for ServiceClass and
// ServicePlan, they are part of the payload returned from the Broker.
func validateExternalID(value string) []string {
	var errs []string
	if len(value) > guidMaxLength {
		errs = append(errs, utilvalidation.MaxLenError(guidMaxLength))
	}
	if !guidRegexp.MatchString(value) {
		errs = append(errs, utilvalidation.RegexError(guidFmt, "my-name", "123-abc", "456-DEF"))
	}
	return errs
}

// ValidateServiceClass validates a ServiceClass and returns a list of errors.
func ValidateServiceClass(serviceclass *sc.ServiceClass) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs,
		apivalidation.ValidateObjectMeta(
			&serviceclass.ObjectMeta,
			false, /* namespace required */
			validateServiceClassName,
			field.NewPath("metadata"))...)

	if "" == serviceclass.BrokerName {
		allErrs = append(allErrs, field.Required(field.NewPath("brokerName"), "brokerName is required"))
	}

	if "" == serviceclass.ExternalID {
		allErrs = append(allErrs, field.Required(field.NewPath("externalID"), "externalID is required"))
	}

	if "" == serviceclass.Description {
		allErrs = append(allErrs, field.Required(field.NewPath("description"), "description is required"))
	}

	for _, msg := range validateExternalID(serviceclass.ExternalID) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("externalID"), serviceclass.ExternalID, msg))
	}

	if len(serviceclass.Plans) < 1 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("plans"), serviceclass.Plans, "at least one plan is required"))
	}

	planNames := sets.NewString()
	for i, plan := range serviceclass.Plans {
		planPath := field.NewPath("plans").Index(i)
		allErrs = append(allErrs, validateServicePlan(plan, planPath)...)

		if planNames.Has(plan.Name) {
			allErrs = append(allErrs, field.Invalid(planPath.Child("name"), plan.Name, "each plan must have a unique name"))
		}
	}

	return allErrs
}

// validateServicePlan validates the fields of a single ServicePlan and
// returns a list of errors.
func validateServicePlan(plan sc.ServicePlan, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, msg := range validateServicePlanName(plan.Name) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), plan.Name, msg))
	}

	if "" == plan.ExternalID {
		allErrs = append(allErrs, field.Required(fldPath.Child("externalID"), "externalID is required"))
	}

	if "" == plan.Description {
		allErrs = append(allErrs, field.Required(field.NewPath("description"), "description is required"))
	}

	for _, msg := range validateExternalID(plan.ExternalID) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("externalID"), plan.ExternalID, msg))
	}

	return allErrs
}

const servicePlanNameFmt string = `[-_a-zA-Z0-9]+`
const servicePlanNameMaxLength int = 63

var servicePlanNameRegexp = regexp.MustCompile("^" + servicePlanNameFmt + "$")

// validateServicePlanName is the validation function for ServicePlan names.
func validateServicePlanName(value string) []string {
	var errs []string
	if len(value) > servicePlanNameMaxLength {
		errs = append(errs, utilvalidation.MaxLenError(servicePlanNameMaxLength))
	}
	if !servicePlanNameRegexp.MatchString(value) {
		errs = append(errs, utilvalidation.RegexError(servicePlanNameFmt, "plan-name", "plan_name"))
	}

	return errs
}

// ValidateServiceClassUpdate checks that when changing from an older
// ServiceClass to a newer ServiceClass is okay.
func ValidateServiceClassUpdate(new *sc.ServiceClass, old *sc.ServiceClass) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateServiceClass(new)...)
	allErrs = append(allErrs, ValidateServiceClass(old)...)

	return allErrs
}
