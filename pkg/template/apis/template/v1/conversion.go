package v1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/api/extension"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc("v1", "Template",
		oapi.GetFieldLabelConversionFunc(templateapi.TemplateToSelectableFields(&templateapi.Template{}), nil),
	); err != nil {
		return err
	}
	if err := scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.String(), "Template",
		oapi.GetFieldLabelConversionFunc(templateapi.TemplateToSelectableFields(&templateapi.Template{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "TemplateInstance",
		oapi.GetFieldLabelConversionFunc(templateapi.TemplateInstanceToSelectableFields(&templateapi.TemplateInstance{}), nil),
	); err != nil {
		return err
	}
	if err := scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.String(), "TemplateInstance",
		oapi.GetFieldLabelConversionFunc(templateapi.TemplateInstanceToSelectableFields(&templateapi.TemplateInstance{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "BrokerTemplateInstance",
		oapi.GetFieldLabelConversionFunc(templateapi.BrokerTemplateInstanceToSelectableFields(&templateapi.BrokerTemplateInstance{}), nil),
	); err != nil {
		return err
	}
	if err := scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.String(), "BrokerTemplateInstance",
		oapi.GetFieldLabelConversionFunc(templateapi.BrokerTemplateInstanceToSelectableFields(&templateapi.BrokerTemplateInstance{}), nil),
	); err != nil {
		return err
	}

	return nil

}

var _ runtime.NestedObjectDecoder = &Template{}
var _ runtime.NestedObjectEncoder = &Template{}

// DecodeNestedObjects decodes the object as a runtime.Unknown with JSON content.
func (c *Template) DecodeNestedObjects(d runtime.Decoder) error {
	for i := range c.Objects {
		if c.Objects[i].Object != nil {
			continue
		}
		c.Objects[i].Object = &runtime.Unknown{
			ContentType: "application/json",
			Raw:         c.Objects[i].Raw,
		}
	}
	return nil
}
func (c *Template) EncodeNestedObjects(e runtime.Encoder) error {
	for i := range c.Objects {
		if err := extension.EncodeNestedRawExtension(unstructured.UnstructuredJSONScheme, &c.Objects[i]); err != nil {
			return err
		}
	}
	return nil
}
