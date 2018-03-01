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

const servicePlanNameFmt string = `[-a-zA-Z0-9]+`
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

// ValidateClusterServicePlan validates a ClusterServicePlan and returns a list of errors.
func ValidateClusterServicePlan(serviceplan *sc.ClusterServicePlan) field.ErrorList {
	return internalValidateClusterServicePlan(serviceplan)
}

func internalValidateClusterServicePlan(serviceplan *sc.ClusterServicePlan) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs,
		apivalidation.ValidateObjectMeta(
			&serviceplan.ObjectMeta,
			false, /* namespace required */
			validateServicePlanName,
			field.NewPath("metadata"))...)

	allErrs = append(allErrs, validateClusterServicePlanSpec(&serviceplan.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateClusterServicePlanSpec(spec *sc.ClusterServicePlanSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if "" == spec.ClusterServiceBrokerName {
		allErrs = append(allErrs, field.Required(fldPath.Child("clusterServiceBrokerName"), "clusterServiceBrokerName is required"))
	}

	if "" == spec.ExternalID {
		allErrs = append(allErrs, field.Required(fldPath.Child("externalID"), "externalID is required"))
	}

	if "" == spec.Description {
		allErrs = append(allErrs, field.Required(fldPath.Child("description"), "description is required"))
	}

	if "" == spec.ClusterServiceClassRef.Name {
		allErrs = append(allErrs, field.Required(fldPath.Child("clusterServiceClassRef"), "an owning serviceclass is required"))
	}

	for _, msg := range validateServicePlanName(spec.ExternalName, false /* prefix */) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("externalName"), spec.ExternalName, msg))
	}

	for _, msg := range validateExternalID(spec.ExternalID) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("externalID"), spec.ExternalID, msg))
	}

	for _, msg := range validateServiceClassName(spec.ClusterServiceClassRef.Name, false /* prefix */) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("clusterServiceClassRef", "name"), spec.ClusterServiceClassRef.Name, msg))
	}

	return allErrs
}

// ValidateClusterServicePlanUpdate checks that when changing from an older
// ClusterServicePlan to a newer ClusterServicePlan is okay.
func ValidateClusterServicePlanUpdate(new *sc.ClusterServicePlan, old *sc.ClusterServicePlan) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, internalValidateClusterServicePlan(new)...)
	if new.Spec.ExternalID != old.Spec.ExternalID {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("externalID"), new.Spec.ExternalID, "externalID cannot change when updating a ClusterServicePlan"))
	}
	return allErrs
}
