package fake

import (
	template "github.com/openshift/origin/pkg/template/apis/template"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

var parameterizedTemplatesResource = schema.GroupVersionResource{Group: "template.openshift.io", Version: "", Resource: "parameterizedtemplates"}

// Parameterize returns a parameterized template
func (c *FakeTemplates) Parameterize(request *template.ParameterizeTemplateRequest) (*template.Template, error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(parameterizedTemplatesResource, c.ns, request), &template.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*template.Template), err
}