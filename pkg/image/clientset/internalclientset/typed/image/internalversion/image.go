package internalversion

import (
	api "github.com/openshift/origin/pkg/image/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	watch "k8s.io/kubernetes/pkg/watch"
)

// ImagesGetter has a method to return a ImageResourceInterface.
// A group's client should implement this interface.
type ImagesGetter interface {
	Images() ImageResourceInterface
}

// ImageResourceInterface has methods to work with ImageResource resources.
type ImageResourceInterface interface {
	Create(*api.Image) (*api.Image, error)
	Update(*api.Image) (*api.Image, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.Image, error)
	List(opts pkg_api.ListOptions) (*api.ImageList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.Image, err error)
	ImageResourceExpansion
}

// images implements ImageResourceInterface
type images struct {
	client restclient.Interface
}

// newImages returns a Images
func newImages(c *ImageClient) *images {
	return &images{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a image and creates it.  Returns the server's representation of the image, and an error, if there is any.
func (c *images) Create(image *api.Image) (result *api.Image, err error) {
	result = &api.Image{}
	err = c.client.Post().
		Resource("images").
		Body(image).
		Do().
		Into(result)
	return
}

// Update takes the representation of a image and updates it. Returns the server's representation of the image, and an error, if there is any.
func (c *images) Update(image *api.Image) (result *api.Image, err error) {
	result = &api.Image{}
	err = c.client.Put().
		Resource("images").
		Name(image.Name).
		Body(image).
		Do().
		Into(result)
	return
}

// Delete takes name of the image and deletes it. Returns an error if one occurs.
func (c *images) Delete(name string, options *pkg_api.DeleteOptions) error {
	return c.client.Delete().
		Resource("images").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *images) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Resource("images").
		VersionedParams(&listOptions, pkg_api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the image, and returns the corresponding image object, and an error if there is any.
func (c *images) Get(name string) (result *api.Image, err error) {
	result = &api.Image{}
	err = c.client.Get().
		Resource("images").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Images that match those selectors.
func (c *images) List(opts pkg_api.ListOptions) (result *api.ImageList, err error) {
	result = &api.ImageList{}
	err = c.client.Get().
		Resource("images").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested images.
func (c *images) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Resource("images").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched image.
func (c *images) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.Image, err error) {
	result = &api.Image{}
	err = c.client.Patch(pt).
		Resource("images").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
