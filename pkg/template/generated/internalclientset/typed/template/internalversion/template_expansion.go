package internalversion

import (
	template "github.com/openshift/origin/pkg/template/apis/template"
)

// TemplateResourceExpansion adds additional client functions to the Template client
type TemplateResourceExpansion interface {
	Parameterize(*template.ParameterizeTemplateRequest) (*template.Template, error)
}

// Watch returns a watch.Interface that watches the requested templates.
func (c *templates) Parameterize(request *template.ParameterizeTemplateRequest) (*template.Template, error) {
	result := &template.Template{}
	err := c.client.Post().
		Namespace(c.ns).
		Resource("parameterizedtemplates").
		Body(request).
		Do().
		Into(result)
	return result, err
}