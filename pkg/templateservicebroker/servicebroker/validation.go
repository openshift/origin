package servicebroker

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	templatevalidation "github.com/openshift/origin/pkg/template/apis/template/validation"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
)

// ValidateProvisionRequest ensures that a ProvisionRequest is valid, beyond
// the validation carried out by the service broker framework itself.
func ValidateProvisionRequest(preq *api.ProvisionRequest) field.ErrorList {
	var allErrs field.ErrorList

	for key := range preq.Parameters {
		if !templatevalidation.ParameterNameRegexp.MatchString(key) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("parameters", key), key, fmt.Sprintf("does not match %v", templatevalidation.ParameterNameRegexp)))
		}
	}

	return allErrs
}

// ValidateBindRequest ensures that a BindRequest is valid, beyond the
// validation carried out by the service broker framework itself.
func ValidateBindRequest(breq *api.BindRequest) field.ErrorList {
	var allErrs field.ErrorList

	for key := range breq.Parameters {
		if !templatevalidation.ParameterNameRegexp.MatchString(key) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("parameters."+key), key, fmt.Sprintf("does not match %v", templatevalidation.ParameterNameRegexp)))
		}
	}

	return allErrs
}
