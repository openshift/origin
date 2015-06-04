package validation

import (
	"fmt"
	"regexp"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/template/api"
)

var parameterNameExp = regexp.MustCompile(`^[a-zA-Z0-9\_]+$`)

// ValidateParameter tests if required fields in the Parameter are set.
func ValidateParameter(param *api.Parameter) (allErrs fielderrors.ValidationErrorList) {
	if len(param.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
		return
	}
	if !parameterNameExp.MatchString(param.Name) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("name", param.Name, fmt.Sprintf("does not match %v", parameterNameExp)))
	}
	return
}

// ValidateProcessedTemplate tests if required fields in the Template are set for processing
func ValidateProcessedTemplate(template *api.Template) fielderrors.ValidationErrorList {
	return validateTemplateBody(template)
}

// ValidateTemplate tests if required fields in the Template are set.
func ValidateTemplate(template *api.Template) (allErrs fielderrors.ValidationErrorList) {
	allErrs = validation.ValidateObjectMeta(&template.ObjectMeta, true, oapi.GetNameValidationFunc(validation.ValidatePodName)).Prefix("metadata")
	allErrs = append(allErrs, validateTemplateBody(template)...)
	return
}

// ValidateTemplateUpdate tests if required fields in the template are set during an update
func ValidateTemplateUpdate(template, oldTemplate *api.Template) fielderrors.ValidationErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&oldTemplate.ObjectMeta, &template.ObjectMeta).Prefix("metadata")
	return allErrs
}

// validateTemplateBody checks the body of a template.
func validateTemplateBody(template *api.Template) (allErrs fielderrors.ValidationErrorList) {
	for i := range template.Parameters {
		paramErr := ValidateParameter(&template.Parameters[i])
		allErrs = append(allErrs, paramErr.PrefixIndex(i).Prefix("parameters")...)
	}
	allErrs = append(allErrs, validation.ValidateLabels(template.ObjectLabels, "labels")...)
	return
}
