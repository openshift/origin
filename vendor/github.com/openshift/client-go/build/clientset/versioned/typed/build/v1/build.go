package v1

import (
	v1 "github.com/openshift/api/build/v1"
	scheme "github.com/openshift/client-go/build/clientset/versioned/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// BuildsGetter has a method to return a BuildInterface.
// A group's client should implement this interface.
type BuildsGetter interface {
	Builds(namespace string) BuildInterface
}

// BuildInterface has methods to work with Build resources.
type BuildInterface interface {
	Create(*v1.Build) (*v1.Build, error)
	Update(*v1.Build) (*v1.Build, error)
	UpdateStatus(*v1.Build) (*v1.Build, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.Build, error)
	List(opts meta_v1.ListOptions) (*v1.BuildList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Build, err error)
	UpdateDetails(buildName string, build *v1.Build) (*v1.Build, error)
	Clone(buildName string, buildRequest *v1.BuildRequest) (*v1.Build, error)

	BuildExpansion
}

// builds implements BuildInterface
type builds struct {
	client rest.Interface
	ns     string
}

// newBuilds returns a Builds
func newBuilds(c *BuildV1Client, namespace string) *builds {
	return &builds{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the build, and returns the corresponding build object, and an error if there is any.
func (c *builds) Get(name string, options meta_v1.GetOptions) (result *v1.Build, err error) {
	result = &v1.Build{}
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
func (c *builds) List(opts meta_v1.ListOptions) (result *v1.BuildList, err error) {
	result = &v1.BuildList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("builds").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested builds.
func (c *builds) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("builds").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a build and creates it.  Returns the server's representation of the build, and an error, if there is any.
func (c *builds) Create(build *v1.Build) (result *v1.Build, err error) {
	result = &v1.Build{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("builds").
		Body(build).
		Do().
		Into(result)
	return
}

// Update takes the representation of a build and updates it. Returns the server's representation of the build, and an error, if there is any.
func (c *builds) Update(build *v1.Build) (result *v1.Build, err error) {
	result = &v1.Build{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("builds").
		Name(build.Name).
		Body(build).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *builds) UpdateStatus(build *v1.Build) (result *v1.Build, err error) {
	result = &v1.Build{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("builds").
		Name(build.Name).
		SubResource("status").
		Body(build).
		Do().
		Into(result)
	return
}

// Delete takes name of the build and deletes it. Returns an error if one occurs.
func (c *builds) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("builds").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *builds) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("builds").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched build.
func (c *builds) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Build, err error) {
	result = &v1.Build{}
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

// UpdateDetails takes the top resource name and the representation of a build and updates it. Returns the server's representation of the build, and an error, if there is any.
func (c *builds) UpdateDetails(buildName string, build *v1.Build) (result *v1.Build, err error) {
	result = &v1.Build{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("builds").
		Name(buildName).
		SubResource("details").
		Body(build).
		Do().
		Into(result)
	return
}

// Clone takes the representation of a buildRequest and creates it.  Returns the server's representation of the build, and an error, if there is any.
func (c *builds) Clone(buildName string, buildRequest *v1.BuildRequest) (result *v1.Build, err error) {
	result = &v1.Build{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("builds").
		Name(buildName).
		SubResource("clone").
		Body(buildRequest).
		Do().
		Into(result)
	return
}
