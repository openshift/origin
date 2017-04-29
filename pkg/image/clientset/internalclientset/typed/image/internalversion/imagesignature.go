package internalversion

import (
	api "github.com/openshift/origin/pkg/image/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	watch "k8s.io/kubernetes/pkg/watch"
)

// ImageSignaturesGetter has a method to return a ImageSignatureInterface.
// A group's client should implement this interface.
type ImageSignaturesGetter interface {
	ImageSignatures() ImageSignatureInterface
}

// ImageSignatureInterface has methods to work with ImageSignature resources.
type ImageSignatureInterface interface {
	Create(*api.ImageSignature) (*api.ImageSignature, error)
	Update(*api.ImageSignature) (*api.ImageSignature, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.ImageSignature, error)
	List(opts pkg_api.ListOptions) (*api.ImageSignatureList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ImageSignature, err error)
	ImageSignatureExpansion
}

// imageSignatures implements ImageSignatureInterface
type imageSignatures struct {
	client restclient.Interface
}

// newImageSignatures returns a ImageSignatures
func newImageSignatures(c *ImageClient) *imageSignatures {
	return &imageSignatures{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a imageSignature and creates it.  Returns the server's representation of the imageSignature, and an error, if there is any.
func (c *imageSignatures) Create(imageSignature *api.ImageSignature) (result *api.ImageSignature, err error) {
	result = &api.ImageSignature{}
	err = c.client.Post().
		Resource("imagesignatures").
		Body(imageSignature).
		Do().
		Into(result)
	return
}

// Update takes the representation of a imageSignature and updates it. Returns the server's representation of the imageSignature, and an error, if there is any.
func (c *imageSignatures) Update(imageSignature *api.ImageSignature) (result *api.ImageSignature, err error) {
	result = &api.ImageSignature{}
	err = c.client.Put().
		Resource("imagesignatures").
		Name(imageSignature.Name).
		Body(imageSignature).
		Do().
		Into(result)
	return
}

// Delete takes name of the imageSignature and deletes it. Returns an error if one occurs.
func (c *imageSignatures) Delete(name string, options *pkg_api.DeleteOptions) error {
	return c.client.Delete().
		Resource("imagesignatures").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *imageSignatures) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Resource("imagesignatures").
		VersionedParams(&listOptions, pkg_api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the imageSignature, and returns the corresponding imageSignature object, and an error if there is any.
func (c *imageSignatures) Get(name string) (result *api.ImageSignature, err error) {
	result = &api.ImageSignature{}
	err = c.client.Get().
		Resource("imagesignatures").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ImageSignatures that match those selectors.
func (c *imageSignatures) List(opts pkg_api.ListOptions) (result *api.ImageSignatureList, err error) {
	result = &api.ImageSignatureList{}
	err = c.client.Get().
		Resource("imagesignatures").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested imageSignatures.
func (c *imageSignatures) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Resource("imagesignatures").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched imageSignature.
func (c *imageSignatures) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ImageSignature, err error) {
	result = &api.ImageSignature{}
	err = c.client.Patch(pt).
		Resource("imagesignatures").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
