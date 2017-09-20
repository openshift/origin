package internalversion

import (
	"k8s.io/client-go/rest"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
)

// TemplateConfigInterface is an interface for processing template client.
type TemplateProcessorInterface interface {
	Process(*templateapi.Template) (*templateapi.Template, error)
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
func (c *templateProcessor) Process(in *templateapi.Template) (*templateapi.Template, error) {
	template := &templateapi.Template{}
	err := c.client.Post().
		Namespace(c.ns).
		Resource("processedTemplates").
		Body(in).Do().Into(template)
	return template, err
}
