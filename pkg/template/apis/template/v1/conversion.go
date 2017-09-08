package v1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/pkg/api/apihelpers"
)

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
		if err := apihelpers.EncodeNestedRawExtension(unstructured.UnstructuredJSONScheme, &c.Objects[i]); err != nil {
			return err
		}
	}
	return nil
}
