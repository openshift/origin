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

// validateServiceInstanceCredentialName is the validation function for ServiceInstanceCredential names.
var validateServiceInstanceCredentialName = apivalidation.NameIsDNSSubdomain

// ValidateServiceInstanceCredential validates a ServiceInstanceCredential and returns a list of errors.
func ValidateServiceInstanceCredential(binding *sc.ServiceInstanceCredential) field.ErrorList {
	return internalValidateServiceInstanceCredential(binding, true)
}

func internalValidateServiceInstanceCredential(binding *sc.ServiceInstanceCredential, create bool) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&binding.ObjectMeta, true, /*namespace*/
		validateServiceInstanceCredentialName,
		field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateServiceInstanceCredentialSpec(&binding.Spec, field.NewPath("Spec"), create)...)

	return allErrs
}

func validateServiceInstanceCredentialSpec(spec *sc.ServiceInstanceCredentialSpec, fldPath *field.Path, create bool) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, msg := range validateServiceInstanceName(spec.ServiceInstanceRef.Name, false /* prefix */) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("instanceRef", "name"), spec.ServiceInstanceRef.Name, msg))
	}

	for _, msg := range apivalidation.NameIsDNSSubdomain(spec.SecretName, false /* prefix */) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("secretName"), spec.SecretName, msg))
	}

	return allErrs
}

// ValidateServiceInstanceCredentialUpdate checks that when changing from an older binding to a newer binding is okay.
func ValidateServiceInstanceCredentialUpdate(new *sc.ServiceInstanceCredential, old *sc.ServiceInstanceCredential) field.ErrorList {
	return internalValidateServiceInstanceCredential(new, false)
}

// ValidateServiceInstanceCredentialStatusUpdate checks that when changing from an older binding to a newer binding is okay.
func ValidateServiceInstanceCredentialStatusUpdate(new *sc.ServiceInstanceCredential, old *sc.ServiceInstanceCredential) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateServiceInstanceCredentialUpdate(new, old)...)
	return allErrs
}
