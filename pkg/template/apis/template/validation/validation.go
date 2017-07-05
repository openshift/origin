package validation

import (
	"fmt"
	"regexp"

	unversionedvalidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/api"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"
	"k8s.io/kubernetes/pkg/api/validation"

	oapi "github.com/openshift/origin/pkg/api"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	uservalidation "github.com/openshift/origin/pkg/user/apis/user/validation"
)

var ParameterNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// ValidateParameter tests if required fields in the Parameter are set.
func ValidateParameter(param *templateapi.Parameter, fldPath *field.Path) (allErrs field.ErrorList) {
	if len(param.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
		return
	}
	if !ParameterNameRegexp.MatchString(param.Name) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), param.Name, fmt.Sprintf("does not match %v", ParameterNameRegexp)))
	}
	return
}

// ValidateProcessedTemplate tests if required fields in the Template are set for processing
func ValidateProcessedTemplate(template *templateapi.Template) field.ErrorList {
	return validateTemplateBody(template)
}

// ValidateTemplate tests if required fields in the Template are set.
func ValidateTemplate(template *templateapi.Template) (allErrs field.ErrorList) {
	allErrs = validation.ValidateObjectMeta(&template.ObjectMeta, true, oapi.GetNameValidationFunc(validation.ValidatePodName), field.NewPath("metadata"))
	allErrs = append(allErrs, validateTemplateBody(template)...)
	return
}

// ValidateTemplateUpdate tests if required fields in the template are set during an update
func ValidateTemplateUpdate(template, oldTemplate *templateapi.Template) field.ErrorList {
	return validation.ValidateObjectMetaUpdate(&template.ObjectMeta, &oldTemplate.ObjectMeta, field.NewPath("metadata"))
}

// validateTemplateBody checks the body of a template.
func validateTemplateBody(template *templateapi.Template) (allErrs field.ErrorList) {
	for i := range template.Parameters {
		allErrs = append(allErrs, ValidateParameter(&template.Parameters[i], field.NewPath("parameters").Index(i))...)
	}
	allErrs = append(allErrs, unversionedvalidation.ValidateLabels(template.ObjectLabels, field.NewPath("labels"))...)
	return
}

// ValidateTemplateInstance tests if required fields in the TemplateInstance are set.
func ValidateTemplateInstance(templateInstance *templateapi.TemplateInstance) (allErrs field.ErrorList) {
	allErrs = validation.ValidateObjectMeta(&templateInstance.ObjectMeta, true, oapi.GetNameValidationFunc(validation.ValidatePodName), field.NewPath("metadata"))
	for _, err := range ValidateTemplate(&templateInstance.Spec.Template) {
		err.Field = "spec.template." + err.Field
		allErrs = append(allErrs, err)
	}
	if templateInstance.Spec.Secret != nil {
		if templateInstance.Spec.Secret.Name != "" {
			for _, msg := range oapi.GetNameValidationFunc(validation.ValidateSecretName)(templateInstance.Spec.Secret.Name, false) {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec.secret.name"), templateInstance.Spec.Secret.Name, msg))
			}
		} else {
			allErrs = append(allErrs, field.Required(field.NewPath("spec.secret.name"), ""))
		}
	}
	if templateInstance.Spec.Requester == nil {
		allErrs = append(allErrs, field.Required(field.NewPath("spec.requester"), ""))
	} else if templateInstance.Spec.Requester.Username == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("spec.requester.username"), ""))
	} else {
		for _, msg := range oapi.GetNameValidationFunc(uservalidation.ValidateUserName)(templateInstance.Spec.Requester.Username, false) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec.requester.username"), templateInstance.Spec.Requester.Username, msg))
		}
	}
	return
}

// ValidateTemplateInstanceUpdate tests if required fields in the TemplateInstance are set during an update
func ValidateTemplateInstanceUpdate(templateInstance, oldTemplateInstance *templateapi.TemplateInstance) (allErrs field.ErrorList) {
	allErrs = validation.ValidateObjectMetaUpdate(&templateInstance.ObjectMeta, &oldTemplateInstance.ObjectMeta, field.NewPath("metadata"))

	if !kapihelper.Semantic.DeepEqual(templateInstance.Spec, oldTemplateInstance.Spec) {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec"), "field is immutable"))
	}
	return
}

// ValidateBrokerTemplateInstance tests if required fields in the BrokerTemplateInstance are set.
func ValidateBrokerTemplateInstance(brokerTemplateInstance *templateapi.BrokerTemplateInstance) (allErrs field.ErrorList) {
	allErrs = validation.ValidateObjectMeta(&brokerTemplateInstance.ObjectMeta, false, oapi.GetNameValidationFunc(validation.ValidatePodName), field.NewPath("metadata"))
	allErrs = append(allErrs, validateTemplateInstanceReference(&brokerTemplateInstance.Spec.TemplateInstance, field.NewPath("spec.templateInstance"), "TemplateInstance")...)
	allErrs = append(allErrs, validateTemplateInstanceReference(&brokerTemplateInstance.Spec.Secret, field.NewPath("spec.secret"), "Secret")...)
	for _, id := range brokerTemplateInstance.Spec.BindingIDs {
		for _, msg := range nameIsUUID(id, false) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec.bindingIDs"), id, msg))
		}
	}
	return
}

// ValidateBrokerTemplateInstanceUpdate tests if required fields in the BrokerTemplateInstance are set during an update
func ValidateBrokerTemplateInstanceUpdate(brokerTemplateInstance, oldBrokerTemplateInstance *templateapi.BrokerTemplateInstance) (allErrs field.ErrorList) {
	allErrs = validation.ValidateObjectMetaUpdate(&brokerTemplateInstance.ObjectMeta, &oldBrokerTemplateInstance.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, validateTemplateInstanceReference(&brokerTemplateInstance.Spec.TemplateInstance, field.NewPath("spec.templateInstance"), "TemplateInstance")...)
	allErrs = append(allErrs, validateTemplateInstanceReference(&brokerTemplateInstance.Spec.Secret, field.NewPath("spec.secret"), "Secret")...)
	for _, id := range brokerTemplateInstance.Spec.BindingIDs {
		for _, msg := range nameIsUUID(id, false) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec.bindingIDs"), id, msg))
		}
	}
	return
}

var uuidRegex = regexp.MustCompile("^(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")

func nameIsUUID(name string, prefix bool) []string {
	if uuidRegex.MatchString(name) {
		return nil
	}
	return []string{"is not a valid UUID"}
}

func validateTemplateInstanceReference(ref *kapi.ObjectReference, fldPath *field.Path, kind string) (allErrs field.ErrorList) {
	if len(ref.Kind) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), ""))
	} else if ref.Kind != kind {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("kind"), ref.Kind, "must be "+kind))
	}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
	} else {
		for _, msg := range oapi.GetNameValidationFunc(validation.ValidatePodName)(ref.Name, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), ref.Name, msg))
		}
	}

	if len(ref.Namespace) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), ""))
	} else {
		for _, msg := range oapi.GetNameValidationFunc(validation.ValidateNamespaceName)(ref.Namespace, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), ref.Namespace, msg))
		}
	}

	return allErrs
}
