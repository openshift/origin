package internalversion

import (
	image "github.com/openshift/origin/pkg/image/apis/image"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rest "k8s.io/client-go/rest"
)

// ImageSignaturesGetter has a method to return a ImageSignatureInterface.
// A group's client should implement this interface.
type ImageSignaturesGetter interface {
	ImageSignatures() ImageSignatureInterface
}

// ImageSignatureInterface has methods to work with ImageSignature resources.
type ImageSignatureInterface interface {
	Create(*image.ImageSignature) (*image.ImageSignature, error)
	Delete(name string, options *v1.DeleteOptions) error
	ImageSignatureExpansion
}

// imageSignatures implements ImageSignatureInterface
type imageSignatures struct {
	client rest.Interface
}

// newImageSignatures returns a ImageSignatures
func newImageSignatures(c *ImageClient) *imageSignatures {
	return &imageSignatures{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a imageSignature and creates it.  Returns the server's representation of the imageSignature, and an error, if there is any.
func (c *imageSignatures) Create(imageSignature *image.ImageSignature) (result *image.ImageSignature, err error) {
	result = &image.ImageSignature{}
	err = c.client.Post().
		Resource("imagesignatures").
		Body(imageSignature).
		Do().
		Into(result)
	return
}

// Delete takes name of the imageSignature and deletes it. Returns an error if one occurs.
func (c *imageSignatures) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("imagesignatures").
		Name(name).
		Body(options).
		Do().
		Error()
}
