package v1

import (
	"k8s.io/kubernetes/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/api/extension"
	"github.com/openshift/origin/pkg/template/api"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	return scheme.AddFieldLabelConversionFunc("v1", "Template",
		oapi.GetFieldLabelConversionFunc(api.TemplateToSelectableFields(&api.Template{}), nil),
	)
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
		if err := extension.EncodeNestedRawExtension(runtime.UnstructuredJSONScheme, &c.Objects[i]); err != nil {
			return err
		}
	}
	return nil
}
