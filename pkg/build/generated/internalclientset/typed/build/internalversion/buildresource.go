package internalversion

import (
	build "github.com/openshift/origin/pkg/build/apis/build"
	scheme "github.com/openshift/origin/pkg/build/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// BuildsGetter has a method to return a BuildResourceInterface.
// A group's client should implement this interface.
type BuildsGetter interface {
	Builds(namespace string) BuildResourceInterface
}

// BuildResourceInterface has methods to work with BuildResource resources.
type BuildResourceInterface interface {
	Create(*build.Build) (*build.Build, error)
	Update(*build.Build) (*build.Build, error)
	UpdateStatus(*build.Build) (*build.Build, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*build.Build, error)
	List(opts v1.ListOptions) (*build.BuildList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *build.Build, err error)
	UpdateDetails(buildResourceName string, buildResource *build.Build) (*build.Build, error)
	Clone(buildResourceName string, buildRequest *build.BuildRequest) (*build.Build, error)

	BuildResourceExpansion
}

// builds implements BuildResourceInterface
type builds struct {
	client rest.Interface
	ns     string
}

// newBuilds returns a Builds
func newBuilds(c *BuildClient, namespace string) *builds {
	return &builds{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the buildResource, and returns the corresponding buildResource object, and an error if there is any.
func (c *builds) Get(name string, options v1.GetOptions) (result *build.Build, err error) {
	result = &build.Build{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("builds").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Builds that match those selectors.
func (c *builds) List(opts v1.ListOptions) (result *build.BuildList, err error) {
	result = &build.BuildList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("builds").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested builds.
func (c *builds) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("builds").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a buildResource and creates it.  Returns the server's representation of the buildResource, and an error, if there is any.
func (c *builds) Create(buildResource *build.Build) (result *build.Build, err error) {
	result = &build.Build{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("builds").
		Body(buildResource).
		Do().
		Into(result)
	return
}

// Update takes the representation of a buildResource and updates it. Returns the server's representation of the buildResource, and an error, if there is any.
func (c *builds) Update(buildResource *build.Build) (result *build.Build, err error) {
	result = &build.Build{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("builds").
		Name(buildResource.Name).
		Body(buildResource).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *builds) UpdateStatus(buildResource *build.Build) (result *build.Build, err error) {
	result = &build.Build{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("builds").
		Name(buildResource.Name).
		SubResource("status").
		Body(buildResource).
		Do().
		Into(result)
	return
}

// Delete takes name of the buildResource and deletes it. Returns an error if one occurs.
func (c *builds) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("builds").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *builds) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("builds").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched buildResource.
func (c *builds) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *build.Build, err error) {
	result = &build.Build{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("builds").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}

// UpdateDetails takes the top resource name and the representation of a buildResource and updates it. Returns the server's representation of the buildResource, and an error, if there is any.
func (c *builds) UpdateDetails(buildResourceName string, buildResource *build.Build) (result *build.Build, err error) {
	result = &build.Build{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("builds").
		Name(buildResourceName).
		SubResource("details").
		Body(buildResource).
		Do().
		Into(result)
	return
}

// Clone takes the representation of a buildRequest and creates it.  Returns the server's representation of the buildResource, and an error, if there is any.
func (c *builds) Clone(buildResourceName string, buildRequest *build.BuildRequest) (result *build.Build, err error) {
	result = &build.Build{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("builds").
		Name(buildResourceName).
		SubResource("clone").
		Body(buildRequest).
		Do().
		Into(result)
	return
}
