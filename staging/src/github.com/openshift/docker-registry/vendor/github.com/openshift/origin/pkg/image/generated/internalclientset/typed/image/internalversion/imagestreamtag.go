package internalversion

import (
	image "github.com/openshift/origin/pkg/image/apis/image"
	scheme "github.com/openshift/origin/pkg/image/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rest "k8s.io/client-go/rest"
)

// ImageStreamTagsGetter has a method to return a ImageStreamTagInterface.
// A group's client should implement this interface.
type ImageStreamTagsGetter interface {
	ImageStreamTags(namespace string) ImageStreamTagInterface
}

// ImageStreamTagInterface has methods to work with ImageStreamTag resources.
type ImageStreamTagInterface interface {
	Create(*image.ImageStreamTag) (*image.ImageStreamTag, error)
	Update(*image.ImageStreamTag) (*image.ImageStreamTag, error)
	Delete(name string, options *v1.DeleteOptions) error
	Get(name string, options v1.GetOptions) (*image.ImageStreamTag, error)
	ImageStreamTagExpansion
}

// imageStreamTags implements ImageStreamTagInterface
type imageStreamTags struct {
	client rest.Interface
	ns     string
}

// newImageStreamTags returns a ImageStreamTags
func newImageStreamTags(c *ImageClient, namespace string) *imageStreamTags {
	return &imageStreamTags{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the imageStreamTag, and returns the corresponding imageStreamTag object, and an error if there is any.
func (c *imageStreamTags) Get(name string, options v1.GetOptions) (result *image.ImageStreamTag, err error) {
	result = &image.ImageStreamTag{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("imagestreamtags").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Create takes the representation of a imageStreamTag and creates it.  Returns the server's representation of the imageStreamTag, and an error, if there is any.
func (c *imageStreamTags) Create(imageStreamTag *image.ImageStreamTag) (result *image.ImageStreamTag, err error) {
	result = &image.ImageStreamTag{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("imagestreamtags").
		Body(imageStreamTag).
		Do().
		Into(result)
	return
}

// Update takes the representation of a imageStreamTag and updates it. Returns the server's representation of the imageStreamTag, and an error, if there is any.
func (c *imageStreamTags) Update(imageStreamTag *image.ImageStreamTag) (result *image.ImageStreamTag, err error) {
	result = &image.ImageStreamTag{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("imagestreamtags").
		Name(imageStreamTag.Name).
		Body(imageStreamTag).
		Do().
		Into(result)
	return
}

// Delete takes name of the imageStreamTag and deletes it. Returns an error if one occurs.
func (c *imageStreamTags) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("imagestreamtags").
		Name(name).
		Body(options).
		Do().
		Error()
}
