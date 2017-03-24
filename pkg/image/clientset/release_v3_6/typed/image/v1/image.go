package v1

import (
	v1 "github.com/openshift/origin/pkg/image/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	api_v1 "k8s.io/kubernetes/pkg/api/v1"
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
	Create(*v1.Image) (*v1.Image, error)
	Update(*v1.Image) (*v1.Image, error)
	Delete(name string, options *api_v1.DeleteOptions) error
	DeleteCollection(options *api_v1.DeleteOptions, listOptions api_v1.ListOptions) error
	Get(name string) (*v1.Image, error)
	List(opts api_v1.ListOptions) (*v1.ImageList, error)
	Watch(opts api_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Image, err error)
	ImageResourceExpansion
}

// images implements ImageResourceInterface
type images struct {
	client restclient.Interface
}

// newImages returns a Images
func newImages(c *ImageV1Client) *images {
	return &images{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a image and creates it.  Returns the server's representation of the image, and an error, if there is any.
func (c *images) Create(image *v1.Image) (result *v1.Image, err error) {
	result = &v1.Image{}
	err = c.client.Post().
		Resource("images").
		Body(image).
		Do().
		Into(result)
	return
}

// Update takes the representation of a image and updates it. Returns the server's representation of the image, and an error, if there is any.
func (c *images) Update(image *v1.Image) (result *v1.Image, err error) {
	result = &v1.Image{}
	err = c.client.Put().
		Resource("images").
		Name(image.Name).
		Body(image).
		Do().
		Into(result)
	return
}

// Delete takes name of the image and deletes it. Returns an error if one occurs.
func (c *images) Delete(name string, options *api_v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("images").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *images) DeleteCollection(options *api_v1.DeleteOptions, listOptions api_v1.ListOptions) error {
	return c.client.Delete().
		Resource("images").
		VersionedParams(&listOptions, api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the image, and returns the corresponding image object, and an error if there is any.
func (c *images) Get(name string) (result *v1.Image, err error) {
	result = &v1.Image{}
	err = c.client.Get().
		Resource("images").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Images that match those selectors.
func (c *images) List(opts api_v1.ListOptions) (result *v1.ImageList, err error) {
	result = &v1.ImageList{}
	err = c.client.Get().
		Resource("images").
		VersionedParams(&opts, api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested images.
func (c *images) Watch(opts api_v1.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Resource("images").
		VersionedParams(&opts, api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched image.
func (c *images) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Image, err error) {
	result = &v1.Image{}
	err = c.client.Patch(pt).
		Resource("images").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
