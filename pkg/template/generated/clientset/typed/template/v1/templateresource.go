package v1

import (
	v1 "github.com/openshift/origin/pkg/template/apis/template/v1"
	scheme "github.com/openshift/origin/pkg/template/generated/clientset/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Create(*v1.Template) (*v1.Template, error)
	Update(*v1.Template) (*v1.Template, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.Template, error)
	List(opts meta_v1.ListOptions) (*v1.TemplateList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Template, err error)
	TemplateResourceExpansion
}

// templates implements TemplateResourceInterface
type templates struct {
	client rest.Interface
	ns     string
}

// newTemplates returns a Templates
func newTemplates(c *TemplateV1Client, namespace string) *templates {
	return &templates{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a templateResource and creates it.  Returns the server's representation of the templateResource, and an error, if there is any.
func (c *templates) Create(templateResource *v1.Template) (result *v1.Template, err error) {
	result = &v1.Template{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("templates").
		Body(templateResource).
		Do().
		Into(result)
	return
}

// Update takes the representation of a templateResource and updates it. Returns the server's representation of the templateResource, and an error, if there is any.
func (c *templates) Update(templateResource *v1.Template) (result *v1.Template, err error) {
	result = &v1.Template{}
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
func (c *templates) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("templates").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *templates) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("templates").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the templateResource, and returns the corresponding templateResource object, and an error if there is any.
func (c *templates) Get(name string, options meta_v1.GetOptions) (result *v1.Template, err error) {
	result = &v1.Template{}
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
func (c *templates) List(opts meta_v1.ListOptions) (result *v1.TemplateList, err error) {
	result = &v1.TemplateList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("templates").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested templates.
func (c *templates) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("templates").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched templateResource.
func (c *templates) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Template, err error) {
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
