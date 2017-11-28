package v1

import (
	v1 "github.com/openshift/api/image/v1"
	scheme "github.com/openshift/client-go/image/clientset/versioned/scheme"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ImageStreamsGetter has a method to return a ImageStreamInterface.
// A group's client should implement this interface.
type ImageStreamsGetter interface {
	ImageStreams(namespace string) ImageStreamInterface
}

// ImageStreamInterface has methods to work with ImageStream resources.
type ImageStreamInterface interface {
	Create(*v1.ImageStream) (*v1.ImageStream, error)
	Update(*v1.ImageStream) (*v1.ImageStream, error)
	UpdateStatus(*v1.ImageStream) (*v1.ImageStream, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.ImageStream, error)
	List(opts meta_v1.ListOptions) (*v1.ImageStreamList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ImageStream, err error)
	Secrets(imageStreamName string, opts meta_v1.ListOptions) (*core_v1.SecretList, error)

	ImageStreamExpansion
}

// imageStreams implements ImageStreamInterface
type imageStreams struct {
	client rest.Interface
	ns     string
}

// newImageStreams returns a ImageStreams
func newImageStreams(c *ImageV1Client, namespace string) *imageStreams {
	return &imageStreams{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the imageStream, and returns the corresponding imageStream object, and an error if there is any.
func (c *imageStreams) Get(name string, options meta_v1.GetOptions) (result *v1.ImageStream, err error) {
	result = &v1.ImageStream{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("imagestreams").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ImageStreams that match those selectors.
func (c *imageStreams) List(opts meta_v1.ListOptions) (result *v1.ImageStreamList, err error) {
	result = &v1.ImageStreamList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("imagestreams").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested imageStreams.
func (c *imageStreams) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("imagestreams").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a imageStream and creates it.  Returns the server's representation of the imageStream, and an error, if there is any.
func (c *imageStreams) Create(imageStream *v1.ImageStream) (result *v1.ImageStream, err error) {
	result = &v1.ImageStream{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("imagestreams").
		Body(imageStream).
		Do().
		Into(result)
	return
}

// Update takes the representation of a imageStream and updates it. Returns the server's representation of the imageStream, and an error, if there is any.
func (c *imageStreams) Update(imageStream *v1.ImageStream) (result *v1.ImageStream, err error) {
	result = &v1.ImageStream{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("imagestreams").
		Name(imageStream.Name).
		Body(imageStream).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *imageStreams) UpdateStatus(imageStream *v1.ImageStream) (result *v1.ImageStream, err error) {
	result = &v1.ImageStream{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("imagestreams").
		Name(imageStream.Name).
		SubResource("status").
		Body(imageStream).
		Do().
		Into(result)
	return
}

// Delete takes name of the imageStream and deletes it. Returns an error if one occurs.
func (c *imageStreams) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("imagestreams").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *imageStreams) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("imagestreams").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched imageStream.
func (c *imageStreams) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ImageStream, err error) {
	result = &v1.ImageStream{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("imagestreams").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}

// Secrets takes v1.ImageStream name, label and field selectors, and returns the list of Secrets that match those selectors.
func (c *imageStreams) Secrets(imageStreamName string, opts meta_v1.ListOptions) (result *core_v1.SecretList, err error) {
	result = &core_v1.SecretList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("imagestreams").
		Name(imageStreamName).
		SubResource("secrets").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}
