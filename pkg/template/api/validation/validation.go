package validation

import (
	"fmt"
	"regexp"

	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util/validation/field"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/template/api"
	unversionedvalidation "k8s.io/kubernetes/pkg/api/unversioned/validation"
)

var parameterNameExp = regexp.MustCompile(`^[a-zA-Z0-9\_]+$`)

// ValidateParameter tests if required fields in the Parameter are set.
func ValidateParameter(param *api.Parameter, fldPath *field.Path) (allErrs field.ErrorList) {
	if len(param.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
		return
	}
	if !parameterNameExp.MatchString(param.Name) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), param.Name, fmt.Sprintf("does not match %v", parameterNameExp)))
	}
	return
}

// ValidateProcessedTemplate tests if required fields in the Template are set for processing
func ValidateProcessedTemplate(template *api.Template) field.ErrorList {
	return validateTemplateBody(template)
}

// ValidateTemplate tests if required fields in the Template are set.
func ValidateTemplate(template *api.Template) (allErrs field.ErrorList) {
	allErrs = validation.ValidateObjectMeta(&template.ObjectMeta, true, oapi.GetNameValidationFunc(validation.ValidatePodName), field.NewPath("metadata"))
	allErrs = append(allErrs, validateTemplateBody(template)...)
	return
}

// ValidateTemplateUpdate tests if required fields in the template are set during an update
func ValidateTemplateUpdate(template, oldTemplate *api.Template) field.ErrorList {
	return validation.ValidateObjectMetaUpdate(&template.ObjectMeta, &oldTemplate.ObjectMeta, field.NewPath("metadata"))
}

// validateTemplateBody checks the body of a template.
func validateTemplateBody(template *api.Template) (allErrs field.ErrorList) {
	for i := range template.Parameters {
		allErrs = append(allErrs, ValidateParameter(&template.Parameters[i], field.NewPath("parameters").Index(i))...)
	}
	allErrs = append(allErrs, unversionedvalidation.ValidateLabels(template.ObjectLabels, field.NewPath("labels"))...)
	return
}
