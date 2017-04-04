package v1

import (
	v1 "github.com/openshift/origin/pkg/image/api/v1"
	scheme "github.com/openshift/origin/pkg/image/client/clientset_generated/release_v1_5/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ImagesGetter has a method to return a ImageInterface.
// A group's client should implement this interface.
type ImagesGetter interface {
	Images(namespace string) ImageInterface
}

// ImageInterface has methods to work with Image resources.
type ImageInterface interface {
	Create(*v1.Image) (*v1.Image, error)
	Update(*v1.Image) (*v1.Image, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.Image, error)
	List(opts meta_v1.ListOptions) (*v1.ImageList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Image, err error)
	ImageExpansion
}

// images implements ImageInterface
type images struct {
	client rest.Interface
	ns     string
}

// newImages returns a Images
func newImages(c *CoreV1Client, namespace string) *images {
	return &images{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a image and creates it.  Returns the server's representation of the image, and an error, if there is any.
func (c *images) Create(image *v1.Image) (result *v1.Image, err error) {
	result = &v1.Image{}
	err = c.client.Post().
		Namespace(c.ns).
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
		Namespace(c.ns).
		Resource("images").
		Name(image.Name).
		Body(image).
		Do().
		Into(result)
	return
}

// Delete takes name of the image and deletes it. Returns an error if one occurs.
func (c *images) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("images").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *images) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("images").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the image, and returns the corresponding image object, and an error if there is any.
func (c *images) Get(name string, options meta_v1.GetOptions) (result *v1.Image, err error) {
	result = &v1.Image{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("images").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Images that match those selectors.
func (c *images) List(opts meta_v1.ListOptions) (result *v1.ImageList, err error) {
	result = &v1.ImageList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("images").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested images.
func (c *images) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("images").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched image.
func (c *images) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Image, err error) {
	result = &v1.Image{}
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
