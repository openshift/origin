package internalversion

import (
	api "github.com/openshift/origin/pkg/image/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	watch "k8s.io/kubernetes/pkg/watch"
)

// ImageStreamImagesGetter has a method to return a ImageStreamImageInterface.
// A group's client should implement this interface.
type ImageStreamImagesGetter interface {
	ImageStreamImages() ImageStreamImageInterface
}

// ImageStreamImageInterface has methods to work with ImageStreamImage resources.
type ImageStreamImageInterface interface {
	Create(*api.ImageStreamImage) (*api.ImageStreamImage, error)
	Update(*api.ImageStreamImage) (*api.ImageStreamImage, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.ImageStreamImage, error)
	List(opts pkg_api.ListOptions) (*api.ImageStreamImageList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ImageStreamImage, err error)
	ImageStreamImageExpansion
}

// imageStreamImages implements ImageStreamImageInterface
type imageStreamImages struct {
	client restclient.Interface
}

// newImageStreamImages returns a ImageStreamImages
func newImageStreamImages(c *ImageClient) *imageStreamImages {
	return &imageStreamImages{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a imageStreamImage and creates it.  Returns the server's representation of the imageStreamImage, and an error, if there is any.
func (c *imageStreamImages) Create(imageStreamImage *api.ImageStreamImage) (result *api.ImageStreamImage, err error) {
	result = &api.ImageStreamImage{}
	err = c.client.Post().
		Resource("imagestreamimages").
		Body(imageStreamImage).
		Do().
		Into(result)
	return
}

// Update takes the representation of a imageStreamImage and updates it. Returns the server's representation of the imageStreamImage, and an error, if there is any.
func (c *imageStreamImages) Update(imageStreamImage *api.ImageStreamImage) (result *api.ImageStreamImage, err error) {
	result = &api.ImageStreamImage{}
	err = c.client.Put().
		Resource("imagestreamimages").
		Name(imageStreamImage.Name).
		Body(imageStreamImage).
		Do().
		Into(result)
	return
}

// Delete takes name of the imageStreamImage and deletes it. Returns an error if one occurs.
func (c *imageStreamImages) Delete(name string, options *pkg_api.DeleteOptions) error {
	return c.client.Delete().
		Resource("imagestreamimages").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *imageStreamImages) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Resource("imagestreamimages").
		VersionedParams(&listOptions, pkg_api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the imageStreamImage, and returns the corresponding imageStreamImage object, and an error if there is any.
func (c *imageStreamImages) Get(name string) (result *api.ImageStreamImage, err error) {
	result = &api.ImageStreamImage{}
	err = c.client.Get().
		Resource("imagestreamimages").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ImageStreamImages that match those selectors.
func (c *imageStreamImages) List(opts pkg_api.ListOptions) (result *api.ImageStreamImageList, err error) {
	result = &api.ImageStreamImageList{}
	err = c.client.Get().
		Resource("imagestreamimages").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested imageStreamImages.
func (c *imageStreamImages) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Resource("imagestreamimages").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched imageStreamImage.
func (c *imageStreamImages) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ImageStreamImage, err error) {
	result = &api.ImageStreamImage{}
	err = c.client.Patch(pt).
		Resource("imagestreamimages").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
