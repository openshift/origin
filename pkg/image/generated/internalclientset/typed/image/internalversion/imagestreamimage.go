package internalversion

import (
	image "github.com/openshift/origin/pkg/image/apis/image"
	scheme "github.com/openshift/origin/pkg/image/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rest "k8s.io/client-go/rest"
)

// ImageStreamImagesGetter has a method to return a ImageStreamImageInterface.
// A group's client should implement this interface.
type ImageStreamImagesGetter interface {
	ImageStreamImages(namespace string) ImageStreamImageInterface
}

// ImageStreamImageInterface has methods to work with ImageStreamImage resources.
type ImageStreamImageInterface interface {
	Get(name string, options v1.GetOptions) (*image.ImageStreamImage, error)
	ImageStreamImageExpansion
}

// imageStreamImages implements ImageStreamImageInterface
type imageStreamImages struct {
	client rest.Interface
	ns     string
}

// newImageStreamImages returns a ImageStreamImages
func newImageStreamImages(c *ImageClient, namespace string) *imageStreamImages {
	return &imageStreamImages{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the imageStreamImage, and returns the corresponding imageStreamImage object, and an error if there is any.
func (c *imageStreamImages) Get(name string, options v1.GetOptions) (result *image.ImageStreamImage, err error) {
	result = &image.ImageStreamImage{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("imagestreamimages").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}
