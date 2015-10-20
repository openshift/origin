package client

import (
	"fmt"

	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/image/api"
)

// ImageStreamImagesNamespacer has methods to work with ImageStreamImage resources in a namespace
type ImageStreamImagesNamespacer interface {
	ImageStreamImages(namespace string) ImageStreamImageInterface
}

// ImageStreamImageInterface exposes methods on ImageStreamImage resources.
type ImageStreamImageInterface interface {
	Get(name, id string) (*api.ImageStreamImage, error)
	List(label labels.Selector, field fields.Selector) (*api.ImageStreamImageList, error)
}

// imageStreamImages implements ImageStreamImagesNamespacer interface
type imageStreamImages struct {
	r  *Client
	ns string
}

// newImageStreamImages returns an imageStreamImages
func newImageStreamImages(c *Client, namespace string) *imageStreamImages {
	return &imageStreamImages{
		r:  c,
		ns: namespace,
	}
}

// Get finds the specified image by name of an image repository and id.
func (c *imageStreamImages) Get(name, id string) (result *api.ImageStreamImage, err error) {
	result = &api.ImageStreamImage{}
	err = c.r.Get().Namespace(c.ns).Resource("imageStreamImages").Name(fmt.Sprintf("%s@%s", name, id)).Do().Into(result)
	return
}

func (c *imageStreamImages) List(label labels.Selector, field fields.Selector) (result *api.ImageStreamImageList, err error) {
	result = &api.ImageStreamImageList{}
	err = c.r.Get().
		Namespace(c.ns).
		Resource("imageStreamImages").
		LabelsSelectorParam(label).
		FieldsSelectorParam(field).
		Do().
		Into(result)
	return
}
