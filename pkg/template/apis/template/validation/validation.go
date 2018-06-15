package validation

import (
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	"k8s.io/kubernetes/pkg/apis/core/validation"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
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

var ValidateTemplateName = validation.NameIsDNSSubdomain

// ValidateTemplate tests if required fields in the Template are set.
func ValidateTemplate(template *templateapi.Template) (allErrs field.ErrorList) {
	allErrs = validation.ValidateObjectMeta(&template.ObjectMeta, true, ValidateTemplateName, field.NewPath("metadata"))
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
	return
}

var ValidateTemplateInstanceName = validation.NameIsDNSSubdomain

// ValidateTemplateInstance tests if required fields in the TemplateInstance are set.
func ValidateTemplateInstance(templateInstance *templateapi.TemplateInstance) (allErrs field.ErrorList) {
	allErrs = validation.ValidateObjectMeta(&templateInstance.ObjectMeta, true, ValidateTemplateInstanceName, field.NewPath("metadata"))

	// Allow the nested template name and namespace to be empty.  If not empty,
	// the fields should pass validation.
	templateCopy := templateInstance.Spec.Template.DeepCopy()
	if templateCopy.Name == "" {
		templateCopy.Name = "dummy"
	}
	if templateCopy.Namespace == "" {
		templateCopy.Namespace = "dummy"
	}
	for _, err := range ValidateTemplate(templateCopy) {
		err.Field = "spec.template." + err.Field
		allErrs = append(allErrs, err)
	}
	if templateInstance.Spec.Secret != nil {
		if templateInstance.Spec.Secret.Name != "" {
			for _, msg := range validation.ValidateSecretName(templateInstance.Spec.Secret.Name, false) {
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

var ValidateBrokerTemplateInstanceName = validation.NameIsDNSSubdomain

// ValidateBrokerTemplateInstance tests if required fields in the BrokerTemplateInstance are set.
func ValidateBrokerTemplateInstance(brokerTemplateInstance *templateapi.BrokerTemplateInstance) (allErrs field.ErrorList) {
	allErrs = validation.ValidateObjectMeta(&brokerTemplateInstance.ObjectMeta, false, ValidateBrokerTemplateInstanceName, field.NewPath("metadata"))
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
		for _, msg := range ValidateTemplateName(ref.Name, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), ref.Name, msg))
		}
	}

	if len(ref.Namespace) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), ""))
	} else {
		for _, msg := range validation.ValidateNamespaceName(ref.Namespace, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), ref.Namespace, msg))
		}
	}

	return allErrs
}
