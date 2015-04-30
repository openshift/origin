package client

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	templateapi "github.com/openshift/origin/pkg/template/api"
)

// TemplateConfigNamespacer has methods to work with Image resources in a namespace
// TODO: Rename to ProcessedTemplates
type TemplateConfigsNamespacer interface {
	TemplateConfigs(namespace string) TemplateConfigInterface
}

// TemplateConfigInterface exposes methods on Image resources.
type TemplateConfigInterface interface {
	Create(t *templateapi.Template) (*templateapi.Template, error)
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

// resourceName returns templates's URL resource name based on resource version.
// Uses "templateConfigs" as the URL resource name for v1beta1 and v1beta2.
func (c *templateConfigs) resourceName() string {
	if kapi.PreV1Beta3(c.r.APIVersion()) {
		return "templateConfigs"
	}
	return "processedTemplates"
}

// Create process the Template and returns its current state
func (c *templateConfigs) Create(in *templateapi.Template) (*templateapi.Template, error) {
	if kapi.PreV1Beta3(c.r.APIVersion()) {
		// TODO: path is deprecated, remove
		config := &kapi.List{}
		if err := c.r.Post().Namespace(c.ns).Resource(c.resourceName()).Body(in).Do().Into(config); err != nil {
			return nil, err
		}
		// TODO: support copy instead of mutating the in, once we have a way to deep copy runtime.Object
		// in upstream.
		/*t, err := kapi.Scheme.Copy(in)
		if err != nil {
			return nil, err
		}*/
		template := in
		template.Objects = config.Items
		return template, nil
	}
	template := &templateapi.Template{}
	err := c.r.Post().Namespace(c.ns).Resource(c.resourceName()).Body(in).Do().Into(template)
	return template, err
}
