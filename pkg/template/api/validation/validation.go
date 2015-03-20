package validation

import (
	"fmt"
	"regexp"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"

	"github.com/openshift/origin/pkg/template/api"
)

var parameterNameExp = regexp.MustCompile(`^[a-zA-Z0-9\_]+$`)

// ValidateParameter tests if required fields in the Parameter are set.
func ValidateParameter(param *api.Parameter) (errs errors.ValidationErrorList) {
	if len(param.Name) == 0 {
		errs = append(errs, errors.NewFieldRequired("name"))
		return
	}
	if !parameterNameExp.MatchString(param.Name) {
		errs = append(errs, errors.NewFieldInvalid("name", param.Name, fmt.Sprintf("does not match %v", parameterNameExp)))
	}
	return
}

// ValidateProcessedTemplate tests if required fields in the Template are set for processing
func ValidateProcessedTemplate(template *api.Template) errors.ValidationErrorList {
	return validateTemplateBody(template)
}

// ValidateTemplate tests if required fields in the Template are set.
func ValidateTemplate(template *api.Template) (errs errors.ValidationErrorList) {
	errs = validation.ValidateObjectMeta(&template.ObjectMeta, true, validation.ValidatePodName).Prefix("metadata")
	errs = append(errs, validateTemplateBody(template)...)
	return
}

// ValidateTemplateUpdate tests if required fields in the template are set during an update
func ValidateTemplateUpdate(oldTemplate, template *api.Template) errors.ValidationErrorList {
	errs := validation.ValidateObjectMetaUpdate(&oldTemplate.ObjectMeta, &template.ObjectMeta).Prefix("metadata")
	return errs
}

// validateTemplateBody checks the body of a template.
func validateTemplateBody(template *api.Template) (errs errors.ValidationErrorList) {
	for i := range template.Parameters {
		paramErr := ValidateParameter(&template.Parameters[i])
		errs = append(errs, paramErr.PrefixIndex(i).Prefix("parameters")...)
	}
	errs = append(errs, validation.ValidateLabels(template.ObjectLabels, "labels")...)
	return
}
