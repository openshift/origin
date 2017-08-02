package internalversion

import (
	template "github.com/openshift/origin/pkg/template/apis/template"
	scheme "github.com/openshift/origin/pkg/template/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// TemplatesGetter has a method to return a TemplateResourceInterface.
// A group's client should implement this interface.
type TemplatesGetter interface {
	Templates(namespace string) TemplateResourceInterface
}

// TemplateResourceInterface has methods to work with TemplateResource resources.
type TemplateResourceInterface interface {
	Create(*template.Template) (*template.Template, error)
	Update(*template.Template) (*template.Template, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*template.Template, error)
	List(opts v1.ListOptions) (*template.TemplateList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *template.Template, err error)
	TemplateResourceExpansion
}

// templates implements TemplateResourceInterface
type templates struct {
	client rest.Interface
	ns     string
}

// newTemplates returns a Templates
func newTemplates(c *TemplateClient, namespace string) *templates {
	return &templates{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the templateResource, and returns the corresponding templateResource object, and an error if there is any.
func (c *templates) Get(name string, options v1.GetOptions) (result *template.Template, err error) {
	result = &template.Template{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("templates").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Templates that match those selectors.
func (c *templates) List(opts v1.ListOptions) (result *template.TemplateList, err error) {
	result = &template.TemplateList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("templates").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested templates.
func (c *templates) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("templates").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a templateResource and creates it.  Returns the server's representation of the templateResource, and an error, if there is any.
func (c *templates) Create(templateResource *template.Template) (result *template.Template, err error) {
	result = &template.Template{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("templates").
		Body(templateResource).
		Do().
		Into(result)
	return
}

// Update takes the representation of a templateResource and updates it. Returns the server's representation of the templateResource, and an error, if there is any.
func (c *templates) Update(templateResource *template.Template) (result *template.Template, err error) {
	result = &template.Template{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("templates").
		Name(templateResource.Name).
		Body(templateResource).
		Do().
		Into(result)
	return
}

// Delete takes name of the templateResource and deletes it. Returns an error if one occurs.
func (c *templates) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("templates").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *templates) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("templates").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched templateResource.
func (c *templates) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *template.Template, err error) {
	result = &template.Template{}
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
