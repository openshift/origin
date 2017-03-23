package servicebroker

import (
	"fmt"

	"github.com/openshift/origin/pkg/openservicebroker/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	templatevalidation "github.com/openshift/origin/pkg/template/api/validation"
	uservalidation "github.com/openshift/origin/pkg/user/api/validation"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util/validation/field"
)

func ValidateProvisionRequest(preq *api.ProvisionRequest) field.ErrorList {
	var allErrs field.ErrorList

	for key := range preq.Parameters {
		if !templatevalidation.ParameterNameRegexp.MatchString(key) &&
			key != templateapi.NamespaceParameterKey &&
			key != templateapi.RequesterUsernameParameterKey {
			allErrs = append(allErrs, field.Invalid(field.NewPath("parameters."+key), key, fmt.Sprintf("does not match %v", templatevalidation.ParameterNameRegexp)))
		}
	}

	allErrs = append(allErrs, validateParameter(templateapi.NamespaceParameterKey, preq.Parameters[templateapi.NamespaceParameterKey], validation.ValidateNamespaceName)...)
	allErrs = append(allErrs, validateParameter(templateapi.RequesterUsernameParameterKey, preq.Parameters[templateapi.RequesterUsernameParameterKey], uservalidation.ValidateUserName)...)

	return allErrs
}

func ValidateBindRequest(breq *api.BindRequest) field.ErrorList {
	var allErrs field.ErrorList

	for key := range breq.Parameters {
		if !templatevalidation.ParameterNameRegexp.MatchString(key) &&
			key != templateapi.RequesterUsernameParameterKey {
			allErrs = append(allErrs, field.Invalid(field.NewPath("parameters."+key), key, fmt.Sprintf("does not match %v", templatevalidation.ParameterNameRegexp)))
		}
	}

	allErrs = append(allErrs, validateParameter(templateapi.RequesterUsernameParameterKey, breq.Parameters[templateapi.RequesterUsernameParameterKey], uservalidation.ValidateUserName)...)

	return allErrs
}

func validateParameter(key, value string, validator func(string, bool) []string) field.ErrorList {
	var allErrs field.ErrorList

	if len(value) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("parameters", key), ""))
	} else {
		for _, err := range validator(value, false) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("parameters", key), value, err))
		}
	}

	return allErrs
}
