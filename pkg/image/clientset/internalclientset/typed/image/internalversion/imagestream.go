package internalversion

import (
	api "github.com/openshift/origin/pkg/image/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	watch "k8s.io/kubernetes/pkg/watch"
)

// ImageStreamsGetter has a method to return a ImageStreamInterface.
// A group's client should implement this interface.
type ImageStreamsGetter interface {
	ImageStreams(namespace string) ImageStreamInterface
}

// ImageStreamInterface has methods to work with ImageStream resources.
type ImageStreamInterface interface {
	Create(*api.ImageStream) (*api.ImageStream, error)
	Update(*api.ImageStream) (*api.ImageStream, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.ImageStream, error)
	List(opts pkg_api.ListOptions) (*api.ImageStreamList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ImageStream, err error)
	ImageStreamExpansion
}

// imageStreams implements ImageStreamInterface
type imageStreams struct {
	client restclient.Interface
	ns     string
}

// newImageStreams returns a ImageStreams
func newImageStreams(c *ImageClient, namespace string) *imageStreams {
	return &imageStreams{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a imageStream and creates it.  Returns the server's representation of the imageStream, and an error, if there is any.
func (c *imageStreams) Create(imageStream *api.ImageStream) (result *api.ImageStream, err error) {
	result = &api.ImageStream{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("imagestreams").
		Body(imageStream).
		Do().
		Into(result)
	return
}

// Update takes the representation of a imageStream and updates it. Returns the server's representation of the imageStream, and an error, if there is any.
func (c *imageStreams) Update(imageStream *api.ImageStream) (result *api.ImageStream, err error) {
	result = &api.ImageStream{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("imagestreams").
		Name(imageStream.Name).
		Body(imageStream).
		Do().
		Into(result)
	return
}

// Delete takes name of the imageStream and deletes it. Returns an error if one occurs.
func (c *imageStreams) Delete(name string, options *pkg_api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("imagestreams").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *imageStreams) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("imagestreams").
		VersionedParams(&listOptions, pkg_api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the imageStream, and returns the corresponding imageStream object, and an error if there is any.
func (c *imageStreams) Get(name string) (result *api.ImageStream, err error) {
	result = &api.ImageStream{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("imagestreams").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ImageStreams that match those selectors.
func (c *imageStreams) List(opts pkg_api.ListOptions) (result *api.ImageStreamList, err error) {
	result = &api.ImageStreamList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("imagestreams").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested imageStreams.
func (c *imageStreams) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("imagestreams").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched imageStream.
func (c *imageStreams) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ImageStream, err error) {
	result = &api.ImageStream{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("imagestreams").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
