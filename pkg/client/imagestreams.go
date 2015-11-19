package client

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ImageStreamsNamespacer has methods to work with ImageStream resources in a namespace
type ImageStreamsNamespacer interface {
	ImageStreams(namespace string) ImageStreamInterface
}

// ImageStreamInterface exposes methods on ImageStream resources.
type ImageStreamInterface interface {
	List(label labels.Selector, field fields.Selector) (*imageapi.ImageStreamList, error)
	Get(name string) (*imageapi.ImageStream, error)
	Create(stream *imageapi.ImageStream) (*imageapi.ImageStream, error)
	Update(stream *imageapi.ImageStream) (*imageapi.ImageStream, error)
	Delete(name string) error
	Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
	UpdateStatus(stream *imageapi.ImageStream) (*imageapi.ImageStream, error)
}

// ImageStreamNamespaceGetter exposes methods to get ImageStreams by Namespace
type ImageStreamNamespaceGetter interface {
	GetByNamespace(namespace, name string) (*imageapi.ImageStream, error)
}

// imageStreams implements ImageStreamsNamespacer interface
type imageStreams struct {
	r  *Client
	ns string
}

// newImageStreams returns an imageStreams
func newImageStreams(c *Client, namespace string) *imageStreams {
	return &imageStreams{
		r:  c,
		ns: namespace,
	}
}

// List returns a list of image streams that match the label and field selectors.
func (c *imageStreams) List(label labels.Selector, field fields.Selector) (result *imageapi.ImageStreamList, err error) {
	result = &imageapi.ImageStreamList{}
	err = c.r.Get().
		Namespace(c.ns).
		Resource("imageStreams").
		LabelsSelectorParam(label).
		FieldsSelectorParam(field).
		Do().
		Into(result)
	return
}

// Get returns information about a particular image stream and error if one occurs.
func (c *imageStreams) Get(name string) (result *imageapi.ImageStream, err error) {
	result = &imageapi.ImageStream{}
	err = c.r.Get().Namespace(c.ns).Resource("imageStreams").Name(name).Do().Into(result)
	return
}

// GetByNamespace returns information about a particular image stream in a particular namespace and error if one occurs.
func (c *imageStreams) GetByNamespace(namespace, name string) (result *imageapi.ImageStream, err error) {
	result = &imageapi.ImageStream{}
	c.r.Get().Namespace(namespace).Resource("imageStreams").Name(name).Do().Into(result)
	return
}

// Create create a new image stream. Returns the server's representation of the image stream and error if one occurs.
func (c *imageStreams) Create(stream *imageapi.ImageStream) (result *imageapi.ImageStream, err error) {
	result = &imageapi.ImageStream{}
	err = c.r.Post().Namespace(c.ns).Resource("imageStreams").Body(stream).Do().Into(result)
	return
}

// Update updates the image stream on the server. Returns the server's representation of the image stream and error if one occurs.
func (c *imageStreams) Update(stream *imageapi.ImageStream) (result *imageapi.ImageStream, err error) {
	result = &imageapi.ImageStream{}
	err = c.r.Put().Namespace(c.ns).Resource("imageStreams").Name(stream.Name).Body(stream).Do().Into(result)
	return
}

// Delete deletes an image stream, returns error if one occurs.
func (c *imageStreams) Delete(name string) (err error) {
	err = c.r.Delete().Namespace(c.ns).Resource("imageStreams").Name(name).Do().Error()
	return
}

// Watch returns a watch.Interface that watches the requested image streams.
func (c *imageStreams) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return c.r.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("imageStreams").
		Param("resourceVersion", resourceVersion).
		LabelsSelectorParam(label).
		FieldsSelectorParam(field).
		Watch()
}

// UpdateStatus updates the image stream's status. Returns the server's representation of the image stream, and an error, if it occurs.
func (c *imageStreams) UpdateStatus(stream *imageapi.ImageStream) (result *imageapi.ImageStream, err error) {
	result = &imageapi.ImageStream{}
	err = c.r.Put().Namespace(c.ns).Resource("imageStreams").Name(stream.Name).SubResource("status").Body(stream).Do().Into(result)
	return
}
