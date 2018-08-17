package v1

import (
	"k8s.io/client-go/rest"

	templatev1 "github.com/openshift/api/template/v1"
)

// TemplateConfigInterface is an interface for processing template client.
type TemplateProcessorInterface interface {
	Process(*templatev1.Template) (*templatev1.Template, error)
}

// NewTemplateProcessorClient returns a client capable of processing the templates.
func NewTemplateProcessorClient(c rest.Interface, ns string) TemplateProcessorInterface {
	return &templateProcessor{client: c, ns: ns}
}

type templateProcessor struct {
	client rest.Interface
	ns     string
}

// Process takes an unprocessed template and returns a processed
// template with all parameters substituted.
func (c *templateProcessor) Process(in *templatev1.Template) (*templatev1.Template, error) {
	template := &templatev1.Template{}
	err := c.client.Post().
		Namespace(c.ns).
		Resource("processedTemplates").
		Body(in).Do().Into(template)
	return template, err
}
