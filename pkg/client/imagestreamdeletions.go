package client

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ImageStreamDeletionsNamespacer has methods to work with ImageStreamDeletion resources in a namespace
type ImageStreamDeletionsNamespacer interface {
	ImageStreamDeletions(namespace string) ImageStreamDeletionInterface
}

// ImageStreamDeletionInterface exposes methods on ImageStreamDeletion resources.
type ImageStreamDeletionInterface interface {
	List(label labels.Selector, field fields.Selector) (*imageapi.ImageStreamDeletionList, error)
	Get(name string) (*imageapi.ImageStreamDeletion, error)
	Create(stream *imageapi.ImageStreamDeletion) (*imageapi.ImageStreamDeletion, error)
	Delete(name string) error
}

// ImageStreamDeletionNamespaceGetter exposes methods to get ImageStreamDeletions by Namespace
type ImageStreamDeletionNamespaceGetter interface {
	GetByNamespace(namespace, name string) (*imageapi.ImageStreamDeletion, error)
}

// imageStreamDeletions implements ImageStreamDeletionsNamespacer interface
type imageStreamDeletions struct {
	r  *Client
	ns string
}

// newImageStreamDeletions returns an imageStreamDeletions
func newImageStreamDeletions(c *Client, namespace string) *imageStreamDeletions {
	return &imageStreamDeletions{
		r:  c,
		ns: namespace,
	}
}

// List returns a list of image stream deletions that match the label and field selectors.
func (c *imageStreamDeletions) List(label labels.Selector, field fields.Selector) (result *imageapi.ImageStreamDeletionList, err error) {
	result = &imageapi.ImageStreamDeletionList{}
	err = c.r.Get().
		Namespace(c.ns).
		Resource("imageStreamDeletions").
		LabelsSelectorParam(label).
		FieldsSelectorParam(field).
		Do().
		Into(result)
	return
}

// Get returns information about a particular image stream deletion and error
// if one occurs.
func (c *imageStreamDeletions) Get(name string) (result *imageapi.ImageStreamDeletion, err error) {
	result = &imageapi.ImageStreamDeletion{}
	err = c.r.Get().Namespace(c.ns).Resource("imageStreamDeletions").Name(name).Do().Into(result)
	return
}

// GetByNamespace returns information about a particular image stream deletion
// in a particular namespace and error if one occurs.
func (c *imageStreamDeletions) GetByNamespace(namespace, name string) (result *imageapi.ImageStreamDeletion, err error) {
	result = &imageapi.ImageStreamDeletion{}
	c.r.Get().Namespace(namespace).Resource("imageStreamDeletions").Name(name).Do().Into(result)
	return
}

// Create creates a new image stream deletion. Returns the server's
// representation of the image stream deletion and error if one occurs.
func (c *imageStreamDeletions) Create(stream *imageapi.ImageStreamDeletion) (result *imageapi.ImageStreamDeletion, err error) {
	result = &imageapi.ImageStreamDeletion{}
	err = c.r.Post().Namespace(c.ns).Resource("imageStreamDeletions").Body(stream).Do().Into(result)
	return
}

// Delete deletes an image stream deletion, returns error if one occurs.
func (c *imageStreamDeletions) Delete(name string) (err error) {
	err = c.r.Delete().Namespace(c.ns).Resource("imageStreamDeletions").Name(name).Do().Error()
	return
}
