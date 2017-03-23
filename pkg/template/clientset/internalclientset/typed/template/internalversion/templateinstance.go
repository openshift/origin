package internalversion

import (
	api "github.com/openshift/origin/pkg/template/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	watch "k8s.io/kubernetes/pkg/watch"
)

// TemplateInstancesGetter has a method to return a TemplateInstanceInterface.
// A group's client should implement this interface.
type TemplateInstancesGetter interface {
	TemplateInstances(namespace string) TemplateInstanceInterface
}

// TemplateInstanceInterface has methods to work with TemplateInstance resources.
type TemplateInstanceInterface interface {
	Create(*api.TemplateInstance) (*api.TemplateInstance, error)
	Update(*api.TemplateInstance) (*api.TemplateInstance, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.TemplateInstance, error)
	List(opts pkg_api.ListOptions) (*api.TemplateInstanceList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.TemplateInstance, err error)
	TemplateInstanceExpansion
}

// templateInstances implements TemplateInstanceInterface
type templateInstances struct {
	client restclient.Interface
	ns     string
}

// newTemplateInstances returns a TemplateInstances
func newTemplateInstances(c *TemplateClient, namespace string) *templateInstances {
	return &templateInstances{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a templateInstance and creates it.  Returns the server's representation of the templateInstance, and an error, if there is any.
func (c *templateInstances) Create(templateInstance *api.TemplateInstance) (result *api.TemplateInstance, err error) {
	result = &api.TemplateInstance{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("templateinstances").
		Body(templateInstance).
		Do().
		Into(result)
	return
}

// Update takes the representation of a templateInstance and updates it. Returns the server's representation of the templateInstance, and an error, if there is any.
func (c *templateInstances) Update(templateInstance *api.TemplateInstance) (result *api.TemplateInstance, err error) {
	result = &api.TemplateInstance{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("templateinstances").
		Name(templateInstance.Name).
		Body(templateInstance).
		Do().
		Into(result)
	return
}

// Delete takes name of the templateInstance and deletes it. Returns an error if one occurs.
func (c *templateInstances) Delete(name string, options *pkg_api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("templateinstances").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *templateInstances) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("templateinstances").
		VersionedParams(&listOptions, pkg_api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the templateInstance, and returns the corresponding templateInstance object, and an error if there is any.
func (c *templateInstances) Get(name string) (result *api.TemplateInstance, err error) {
	result = &api.TemplateInstance{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("templateinstances").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of TemplateInstances that match those selectors.
func (c *templateInstances) List(opts pkg_api.ListOptions) (result *api.TemplateInstanceList, err error) {
	result = &api.TemplateInstanceList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("templateinstances").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested templateInstances.
func (c *templateInstances) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("templateinstances").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched templateInstance.
func (c *templateInstances) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.TemplateInstance, err error) {
	result = &api.TemplateInstance{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("templateinstances").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
