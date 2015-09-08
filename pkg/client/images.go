package client

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ImagesInterfacer has methods to work with Image resources
type ImagesInterfacer interface {
	Images() ImageInterface
}

// ImageInterface exposes methods on Image resources.
type ImageInterface interface {
	List(label labels.Selector, field fields.Selector) (*imageapi.ImageList, error)
	Get(name string) (*imageapi.Image, error)
	Create(image *imageapi.Image) (*imageapi.Image, error)
	Delete(name string) error
}

// images implements ImagesInterface.
type images struct {
	r *Client
}

// newImages returns an images
func newImages(c *Client) ImageInterface {
	return &images{
		r: c,
	}
}

// List returns a list of images that match the label and field selectors.
func (c *images) List(label labels.Selector, field fields.Selector) (result *imageapi.ImageList, err error) {
	result = &imageapi.ImageList{}
	err = c.r.Get().
		Resource("images").
		LabelsSelectorParam(label).
		FieldsSelectorParam(field).
		Do().
		Into(result)
	return
}

// Get returns information about a particular image and error if one occurs.
func (c *images) Get(name string) (result *imageapi.Image, err error) {
	result = &imageapi.Image{}
	err = c.r.Get().Resource("images").Name(name).Do().Into(result)
	return
}

// Create creates a new image. Returns the server's representation of the image and error if one occurs.
func (c *images) Create(image *imageapi.Image) (result *imageapi.Image, err error) {
	result = &imageapi.Image{}
	err = c.r.Post().Resource("images").Body(image).Do().Into(result)
	return
}

// Delete deletes an image, returns error if one occurs.
func (c *images) Delete(name string) (err error) {
	err = c.r.Delete().Resource("images").Name(name).Do().Error()
	return
}
