package unversioned

import (
	api "github.com/openshift/origin/pkg/image/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	watch "k8s.io/kubernetes/pkg/watch"
)

// ImagesGetter has a method to return a ImageInterface.
// A group's client should implement this interface.
type ImagesGetter interface {
	Images(namespace string) ImageInterface
}

// ImageInterface has methods to work with Image resources.
type ImageInterface interface {
	Create(*api.Image) (*api.Image, error)
	Update(*api.Image) (*api.Image, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.Image, error)
	List(opts pkg_api.ListOptions) (*api.ImageList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.Image, err error)
	ImageExpansion
}

// images implements ImageInterface
type images struct {
	client *CoreClient
	ns     string
}

// newImages returns a Images
func newImages(c *CoreClient, namespace string) *images {
	return &images{
		client: c,
		ns:     namespace,
	}
}

// Create takes the representation of a image and creates it.  Returns the server's representation of the image, and an error, if there is any.
func (c *images) Create(image *api.Image) (result *api.Image, err error) {
	result = &api.Image{}
	err = c.client.Post().
		Namespace(c.ns).
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
		Namespace(c.ns).
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
		Namespace(c.ns).
		Resource("images").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *images) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
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
		Namespace(c.ns).
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
		Namespace(c.ns).
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
		Namespace(c.ns).
		Resource("images").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched image.
func (c *images) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.Image, err error) {
	result = &api.Image{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("images").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
