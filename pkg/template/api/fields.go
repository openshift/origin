package api

import "k8s.io/kubernetes/pkg/fields"

// TemplateToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func TemplateToSelectableFields(template *Template) fields.Set {
	return fields.Set{
		"metadata.name": template.Name,
	}
}

// TemplateInstanceToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func TemplateInstanceToSelectableFields(templateInstance *TemplateInstance) fields.Set {
	return fields.Set{
		"metadata.name":      templateInstance.Name,
		"metadata.namespace": templateInstance.Namespace,
	}
}

// BrokerTemplateInstanceToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func BrokerTemplateInstanceToSelectableFields(brokertemplateinstance *BrokerTemplateInstance) fields.Set {
	return fields.Set{
		"metadata.name": brokertemplateinstance.Name,
	}
}
