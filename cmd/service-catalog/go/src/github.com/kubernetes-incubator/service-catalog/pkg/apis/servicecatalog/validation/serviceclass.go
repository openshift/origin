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

const serviceClassNameFmt string = `[-a-zA-Z0-9]+`
const serviceClassNameMaxLength int = 63

var serviceClassNameRegexp = regexp.MustCompile("^" + serviceClassNameFmt + "$")

const guidFmt string = "[a-zA-Z0-9]([-a-zA-Z0-9.]*[a-zA-Z0-9])?"
const guidMaxLength int = 63

// guidRegexp is a loosened validation for
// DNS1123 labels that allows uppercase characters.
var guidRegexp = regexp.MustCompile("^" + guidFmt + "$")

// validateServiceClassName is the validation function for Service names.
func validateServiceClassName(value string, prefix bool) []string {
	var errs []string
	if len(value) > serviceClassNameMaxLength {
		errs = append(errs, utilvalidation.MaxLenError(serviceClassNameMaxLength))
	}
	if !serviceClassNameRegexp.MatchString(value) {
		errs = append(errs, utilvalidation.RegexError(serviceClassNameFmt, "service-name-40d-0983-1b89"))
	}

	return errs
}

// validateExternalID is the validation function for External IDs that
// have been passed in. External IDs used to be OpenServiceBrokerAPI
// GUIDs, so we will retain that form until there is another provider
// that desires a different form.  In the case of the OSBAPI we
// generate GUIDs for ServiceInstances and ServiceBindings, but for ClusterServiceClass and
// ServicePlan, they are part of the payload returned from the ClusterServiceBroker.
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

// ValidateClusterServiceClass validates a ClusterServiceClass and returns a list of errors.
func ValidateClusterServiceClass(serviceclass *sc.ClusterServiceClass) field.ErrorList {
	return internalValidateClusterServiceClass(serviceclass)
}

func internalValidateClusterServiceClass(serviceclass *sc.ClusterServiceClass) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs,
		apivalidation.ValidateObjectMeta(
			&serviceclass.ObjectMeta,
			false, /* namespace required */
			validateServiceClassName,
			field.NewPath("metadata"))...)

	allErrs = append(allErrs, validateClusterServiceClassSpec(&serviceclass.Spec, field.NewPath("spec"), true)...)
	return allErrs
}

func validateClusterServiceClassSpec(spec *sc.ClusterServiceClassSpec, fldPath *field.Path, create bool) field.ErrorList {
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

	for _, msg := range validateServiceClassName(spec.ExternalName, false /* prefix */) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("externalName"), spec.ExternalName, msg))
	}
	for _, msg := range validateExternalID(spec.ExternalID) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("externalID"), spec.ExternalID, msg))
	}

	return allErrs
}

// ValidateClusterServiceClassUpdate checks that when changing from an older
// ClusterServiceClass to a newer ClusterServiceClass is okay.
func ValidateClusterServiceClassUpdate(new *sc.ClusterServiceClass, old *sc.ClusterServiceClass) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, internalValidateClusterServiceClass(new)...)

	return allErrs
}
