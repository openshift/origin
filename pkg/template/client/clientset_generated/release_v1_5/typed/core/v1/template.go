package v1

import (
	v1 "github.com/openshift/origin/pkg/template/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	watch "k8s.io/kubernetes/pkg/watch"
)

// TemplatesGetter has a method to return a TemplateInterface.
// A group's client should implement this interface.
type TemplatesGetter interface {
	Templates(namespace string) TemplateInterface
}

// TemplateInterface has methods to work with Template resources.
type TemplateInterface interface {
	Create(*v1.Template) (*v1.Template, error)
	Update(*v1.Template) (*v1.Template, error)
	Delete(name string, options *api.DeleteOptions) error
	DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error
	Get(name string) (*v1.Template, error)
	List(opts api.ListOptions) (*v1.TemplateList, error)
	Watch(opts api.ListOptions) (watch.Interface, error)
	Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Template, err error)
	TemplateExpansion
}

// templates implements TemplateInterface
type templates struct {
	client *CoreClient
	ns     string
}

// newTemplates returns a Templates
func newTemplates(c *CoreClient, namespace string) *templates {
	return &templates{
		client: c,
		ns:     namespace,
	}
}

// Create takes the representation of a template and creates it.  Returns the server's representation of the template, and an error, if there is any.
func (c *templates) Create(template *v1.Template) (result *v1.Template, err error) {
	result = &v1.Template{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("templates").
		Body(template).
		Do().
		Into(result)
	return
}

// Update takes the representation of a template and updates it. Returns the server's representation of the template, and an error, if there is any.
func (c *templates) Update(template *v1.Template) (result *v1.Template, err error) {
	result = &v1.Template{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("templates").
		Name(template.Name).
		Body(template).
		Do().
		Into(result)
	return
}

// Delete takes name of the template and deletes it. Returns an error if one occurs.
func (c *templates) Delete(name string, options *api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("templates").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *templates) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("templates").
		VersionedParams(&listOptions, api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the template, and returns the corresponding template object, and an error if there is any.
func (c *templates) Get(name string) (result *v1.Template, err error) {
	result = &v1.Template{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("templates").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Templates that match those selectors.
func (c *templates) List(opts api.ListOptions) (result *v1.TemplateList, err error) {
	result = &v1.TemplateList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("templates").
		VersionedParams(&opts, api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested templates.
func (c *templates) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("templates").
		VersionedParams(&opts, api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched template.
func (c *templates) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Template, err error) {
	result = &v1.Template{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("templates").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
