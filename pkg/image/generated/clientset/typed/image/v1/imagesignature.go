package v1

import (
	v1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rest "k8s.io/client-go/rest"
)

// ImageSignaturesGetter has a method to return a ImageSignatureInterface.
// A group's client should implement this interface.
type ImageSignaturesGetter interface {
	ImageSignatures(namespace string) ImageSignatureInterface
}

// ImageSignatureInterface has methods to work with ImageSignature resources.
type ImageSignatureInterface interface {
	Create(*v1.ImageSignature) (*v1.ImageSignature, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	ImageSignatureExpansion
}

// imageSignatures implements ImageSignatureInterface
type imageSignatures struct {
	client rest.Interface
	ns     string
}

// newImageSignatures returns a ImageSignatures
func newImageSignatures(c *ImageV1Client, namespace string) *imageSignatures {
	return &imageSignatures{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a imageSignature and creates it.  Returns the server's representation of the imageSignature, and an error, if there is any.
func (c *imageSignatures) Create(imageSignature *v1.ImageSignature) (result *v1.ImageSignature, err error) {
	result = &v1.ImageSignature{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("imagesignatures").
		Body(imageSignature).
		Do().
		Into(result)
	return
}

// Delete takes name of the imageSignature and deletes it. Returns an error if one occurs.
func (c *imageSignatures) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("imagesignatures").
		Name(name).
		Body(options).
		Do().
		Error()
}
