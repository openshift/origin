package v1

import (
	v1 "github.com/openshift/origin/pkg/template/apis/template/v1"
)

// TemplateResourceExpansion adds additional client functions to the Template client
type TemplateResourceExpansion interface {
	Parameterize(*v1.ParameterizeTemplateRequest) (*v1.Template, error)
}

// Watch returns a watch.Interface that watches the requested templates.
func (c *templates) Parameterize(request *v1.ParameterizeTemplateRequest) (*v1.Template, error) {
	result := &v1.Template{}
	err := c.client.Post().
		Namespace(c.ns).
		Resource("parameterizedtemplates").
		Body(request).
		Do().
		Into(result)
	return result, err
}