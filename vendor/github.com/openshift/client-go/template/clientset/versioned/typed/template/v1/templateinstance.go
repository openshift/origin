package v1

import (
	v1 "github.com/openshift/api/template/v1"
	scheme "github.com/openshift/client-go/template/clientset/versioned/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// TemplateInstancesGetter has a method to return a TemplateInstanceInterface.
// A group's client should implement this interface.
type TemplateInstancesGetter interface {
	TemplateInstances(namespace string) TemplateInstanceInterface
}

// TemplateInstanceInterface has methods to work with TemplateInstance resources.
type TemplateInstanceInterface interface {
	Create(*v1.TemplateInstance) (*v1.TemplateInstance, error)
	Update(*v1.TemplateInstance) (*v1.TemplateInstance, error)
	UpdateStatus(*v1.TemplateInstance) (*v1.TemplateInstance, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.TemplateInstance, error)
	List(opts meta_v1.ListOptions) (*v1.TemplateInstanceList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.TemplateInstance, err error)
	TemplateInstanceExpansion
}

// templateInstances implements TemplateInstanceInterface
type templateInstances struct {
	client rest.Interface
	ns     string
}

// newTemplateInstances returns a TemplateInstances
func newTemplateInstances(c *TemplateV1Client, namespace string) *templateInstances {
	return &templateInstances{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the templateInstance, and returns the corresponding templateInstance object, and an error if there is any.
func (c *templateInstances) Get(name string, options meta_v1.GetOptions) (result *v1.TemplateInstance, err error) {
	result = &v1.TemplateInstance{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("templateinstances").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of TemplateInstances that match those selectors.
func (c *templateInstances) List(opts meta_v1.ListOptions) (result *v1.TemplateInstanceList, err error) {
	result = &v1.TemplateInstanceList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("templateinstances").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested templateInstances.
func (c *templateInstances) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("templateinstances").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a templateInstance and creates it.  Returns the server's representation of the templateInstance, and an error, if there is any.
func (c *templateInstances) Create(templateInstance *v1.TemplateInstance) (result *v1.TemplateInstance, err error) {
	result = &v1.TemplateInstance{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("templateinstances").
		Body(templateInstance).
		Do().
		Into(result)
	return
}

// Update takes the representation of a templateInstance and updates it. Returns the server's representation of the templateInstance, and an error, if there is any.
func (c *templateInstances) Update(templateInstance *v1.TemplateInstance) (result *v1.TemplateInstance, err error) {
	result = &v1.TemplateInstance{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("templateinstances").
		Name(templateInstance.Name).
		Body(templateInstance).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *templateInstances) UpdateStatus(templateInstance *v1.TemplateInstance) (result *v1.TemplateInstance, err error) {
	result = &v1.TemplateInstance{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("templateinstances").
		Name(templateInstance.Name).
		SubResource("status").
		Body(templateInstance).
		Do().
		Into(result)
	return
}

// Delete takes name of the templateInstance and deletes it. Returns an error if one occurs.
func (c *templateInstances) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("templateinstances").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *templateInstances) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("templateinstances").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched templateInstance.
func (c *templateInstances) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.TemplateInstance, err error) {
	result = &v1.TemplateInstance{}
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
