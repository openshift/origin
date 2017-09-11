/*
Copyright 2016 The Kubernetes Authors.

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

// ValidateServiceBrokerName is the validation function for Broker names.
var ValidateServiceBrokerName = apivalidation.NameIsDNSSubdomain

// ValidateServiceBroker implements the validation rules for a BrokerResource.
func ValidateServiceBroker(broker *sc.ServiceBroker) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs,
		apivalidation.ValidateObjectMeta(&broker.ObjectMeta,
			false, /* namespace required */
			ValidateServiceBrokerName,
			field.NewPath("metadata"))...)

	allErrs = append(allErrs, validateServiceBrokerSpec(&broker.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateServiceBrokerSpec(spec *sc.ServiceBrokerSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if "" == spec.URL {
		allErrs = append(allErrs,
			field.Required(fldPath.Child("url"),
				"brokers must have a remote url to contact"))
	}

	// if there is auth information, check it to make sure that it's properly formatted
	if spec.AuthInfo != nil {
		if spec.AuthInfo.Basic != nil {
			secretRef := spec.AuthInfo.Basic.SecretRef
			if secretRef != nil {
				for _, msg := range apivalidation.ValidateNamespaceName(secretRef.Namespace, false /* prefix */) {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("authInfo", "basic", "secretRef", "namespace"), secretRef.Namespace, msg))
				}
				for _, msg := range apivalidation.NameIsDNSSubdomain(secretRef.Name, false /* prefix */) {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("authInfo", "basic", "secretRef", "name"), secretRef.Name, msg))
				}
			} else {
				allErrs = append(
					allErrs,
					field.Required(fldPath.Child("authInfo", "basic", "secretRef"), "a basic auth secret is required"),
				)
			}
		} else if spec.AuthInfo.Bearer != nil {
			secretRef := spec.AuthInfo.Bearer.SecretRef
			if secretRef != nil {
				for _, msg := range apivalidation.ValidateNamespaceName(secretRef.Namespace, false /* prefix */) {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("authInfo", "bearer", "secretRef", "namespace"), secretRef.Namespace, msg))
				}
				for _, msg := range apivalidation.NameIsDNSSubdomain(secretRef.Name, false /* prefix */) {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("authInfo", "bearer", "secretRef", "name"), secretRef.Name, msg))
				}
			} else {
				allErrs = append(
					allErrs,
					field.Required(fldPath.Child("authInfo", "bearer", "secretRef"), "a basic auth secret is required"),
				)
			}
		} else if spec.AuthInfo.BasicAuthSecret != nil {
			basicAuthSecret := spec.AuthInfo.BasicAuthSecret
			if basicAuthSecret != nil {
				for _, msg := range apivalidation.ValidateNamespaceName(basicAuthSecret.Namespace, false /* prefix */) {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("authInfo", "basicAuthSecret", "namespace"), basicAuthSecret.Namespace, msg))
				}
				for _, msg := range apivalidation.NameIsDNSSubdomain(basicAuthSecret.Name, false /* prefix */) {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("authInfo", "basicAuthSecret", "name"), basicAuthSecret.Name, msg))
				}
			}
		} else {
			// Authentication
			allErrs = append(
				allErrs,
				field.Required(fldPath.Child("authInfo"), "auth config is required"),
			)
		}

	}

	if spec.InsecureSkipTLSVerify && len(spec.CABundle) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("caBundle"), spec.CABundle, "caBundle cannot be used when insecureSkipTLSVerify is true"))
	}

	return allErrs
}

// ValidateServiceBrokerUpdate checks that when changing from an older broker to a newer broker is okay ?
func ValidateServiceBrokerUpdate(new *sc.ServiceBroker, old *sc.ServiceBroker) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateServiceBroker(new)...)
	allErrs = append(allErrs, ValidateServiceBroker(old)...)
	return allErrs
}

// ValidateServiceBrokerStatusUpdate checks that when changing from an older broker to a newer broker is okay.
func ValidateServiceBrokerStatusUpdate(new *sc.ServiceBroker, old *sc.ServiceBroker) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateServiceBrokerUpdate(new, old)...)
	return allErrs
}
