package fake

import (
	template_v1 "github.com/openshift/origin/pkg/template/apis/template/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

var parameterizedTemplatesResource = schema.GroupVersionResource{Group: "template.openshift.io", Version: "v1", Resource: "parameterizedtemplates"}

// Parameterize returns a parameterized template
func (c *FakeTemplates) Parameterize(request *template_v1.ParameterizeTemplateRequest) (*template_v1.Template, error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(parameterizedTemplatesResource, c.ns, request), &template_v1.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*template_v1.Template), err
}
