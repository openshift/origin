package internalversion

import (
	api "github.com/openshift/origin/pkg/image/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	watch "k8s.io/kubernetes/pkg/watch"
)

// ImageStreamTagsGetter has a method to return a ImageStreamTagInterface.
// A group's client should implement this interface.
type ImageStreamTagsGetter interface {
	ImageStreamTags(namespace string) ImageStreamTagInterface
}

// ImageStreamTagInterface has methods to work with ImageStreamTag resources.
type ImageStreamTagInterface interface {
	Create(*api.ImageStreamTag) (*api.ImageStreamTag, error)
	Update(*api.ImageStreamTag) (*api.ImageStreamTag, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.ImageStreamTag, error)
	List(opts pkg_api.ListOptions) (*api.ImageStreamTagList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ImageStreamTag, err error)
	ImageStreamTagExpansion
}

// imageStreamTags implements ImageStreamTagInterface
type imageStreamTags struct {
	client restclient.Interface
	ns     string
}

// newImageStreamTags returns a ImageStreamTags
func newImageStreamTags(c *ImageClient, namespace string) *imageStreamTags {
	return &imageStreamTags{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a imageStreamTag and creates it.  Returns the server's representation of the imageStreamTag, and an error, if there is any.
func (c *imageStreamTags) Create(imageStreamTag *api.ImageStreamTag) (result *api.ImageStreamTag, err error) {
	result = &api.ImageStreamTag{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("imagestreamtags").
		Body(imageStreamTag).
		Do().
		Into(result)
	return
}

// Update takes the representation of a imageStreamTag and updates it. Returns the server's representation of the imageStreamTag, and an error, if there is any.
func (c *imageStreamTags) Update(imageStreamTag *api.ImageStreamTag) (result *api.ImageStreamTag, err error) {
	result = &api.ImageStreamTag{}
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
func (c *imageStreamTags) Delete(name string, options *pkg_api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("imagestreamtags").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *imageStreamTags) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("imagestreamtags").
		VersionedParams(&listOptions, pkg_api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the imageStreamTag, and returns the corresponding imageStreamTag object, and an error if there is any.
func (c *imageStreamTags) Get(name string) (result *api.ImageStreamTag, err error) {
	result = &api.ImageStreamTag{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("imagestreamtags").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ImageStreamTags that match those selectors.
func (c *imageStreamTags) List(opts pkg_api.ListOptions) (result *api.ImageStreamTagList, err error) {
	result = &api.ImageStreamTagList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("imagestreamtags").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested imageStreamTags.
func (c *imageStreamTags) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("imagestreamtags").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched imageStreamTag.
func (c *imageStreamTags) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ImageStreamTag, err error) {
	result = &api.ImageStreamTag{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("imagestreamtags").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
