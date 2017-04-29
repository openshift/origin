package internalversion

import (
	api "github.com/openshift/origin/pkg/image/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	watch "k8s.io/kubernetes/pkg/watch"
)

// ImageStreamMappingsGetter has a method to return a ImageStreamMappingInterface.
// A group's client should implement this interface.
type ImageStreamMappingsGetter interface {
	ImageStreamMappings(namespace string) ImageStreamMappingInterface
}

// ImageStreamMappingInterface has methods to work with ImageStreamMapping resources.
type ImageStreamMappingInterface interface {
	Create(*api.ImageStreamMapping) (*api.ImageStreamMapping, error)
	Update(*api.ImageStreamMapping) (*api.ImageStreamMapping, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.ImageStreamMapping, error)
	List(opts pkg_api.ListOptions) (*api.ImageStreamMappingList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ImageStreamMapping, err error)
	ImageStreamMappingExpansion
}

// imageStreamMappings implements ImageStreamMappingInterface
type imageStreamMappings struct {
	client restclient.Interface
	ns     string
}

// newImageStreamMappings returns a ImageStreamMappings
func newImageStreamMappings(c *ImageClient, namespace string) *imageStreamMappings {
	return &imageStreamMappings{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a imageStreamMapping and creates it.  Returns the server's representation of the imageStreamMapping, and an error, if there is any.
func (c *imageStreamMappings) Create(imageStreamMapping *api.ImageStreamMapping) (result *api.ImageStreamMapping, err error) {
	result = &api.ImageStreamMapping{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("imagestreammappings").
		Body(imageStreamMapping).
		Do().
		Into(result)
	return
}

// Update takes the representation of a imageStreamMapping and updates it. Returns the server's representation of the imageStreamMapping, and an error, if there is any.
func (c *imageStreamMappings) Update(imageStreamMapping *api.ImageStreamMapping) (result *api.ImageStreamMapping, err error) {
	result = &api.ImageStreamMapping{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("imagestreammappings").
		Name(imageStreamMapping.Name).
		Body(imageStreamMapping).
		Do().
		Into(result)
	return
}

// Delete takes name of the imageStreamMapping and deletes it. Returns an error if one occurs.
func (c *imageStreamMappings) Delete(name string, options *pkg_api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("imagestreammappings").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *imageStreamMappings) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("imagestreammappings").
		VersionedParams(&listOptions, pkg_api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the imageStreamMapping, and returns the corresponding imageStreamMapping object, and an error if there is any.
func (c *imageStreamMappings) Get(name string) (result *api.ImageStreamMapping, err error) {
	result = &api.ImageStreamMapping{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("imagestreammappings").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ImageStreamMappings that match those selectors.
func (c *imageStreamMappings) List(opts pkg_api.ListOptions) (result *api.ImageStreamMappingList, err error) {
	result = &api.ImageStreamMappingList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("imagestreammappings").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested imageStreamMappings.
func (c *imageStreamMappings) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("imagestreammappings").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched imageStreamMapping.
func (c *imageStreamMappings) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ImageStreamMapping, err error) {
	result = &api.ImageStreamMapping{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("imagestreammappings").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
