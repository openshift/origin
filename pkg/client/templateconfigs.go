package client

import (
	configapi "github.com/openshift/origin/pkg/config/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

// TemplateConfigNamespacer has methods to work with Image resources in a namespace
type TemplateConfigsNamespacer interface {
	TemplateConfigs(namespace string) TemplateConfigInterface
}

// TemplateConfigInterface exposes methods on Image resources.
type TemplateConfigInterface interface {
	Create(t *templateapi.Template) (*configapi.Config, error)
}

// templateConfigs implements TemplateConfigsNamespacer interface
type templateConfigs struct {
	r  *Client
	ns string
}

// newTemplateConfigs returns an TemplateConfigInterface
func newTemplateConfigs(c *Client, namespace string) TemplateConfigInterface {
	return &templateConfigs{
		r:  c,
		ns: namespace,
	}
}

// Create process the Template and return Config/List object with substituted
// parameters
func (c *templateConfigs) Create(in *templateapi.Template) (*configapi.Config, error) {
	config := &configapi.Config{}
	err := c.r.Post().Namespace(c.ns).Resource("templateConfigs").Body(in).Do().Into(config)
	return config, err
}
