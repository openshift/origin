package unversioned

import (
	api "github.com/openshift/origin/pkg/build/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	watch "k8s.io/kubernetes/pkg/watch"
)

// BuildsGetter has a method to return a BuildInterface.
// A group's client should implement this interface.
type BuildsGetter interface {
	Builds(namespace string) BuildInterface
}

// BuildInterface has methods to work with Build resources.
type BuildInterface interface {
	Create(*api.Build) (*api.Build, error)
	Update(*api.Build) (*api.Build, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.Build, error)
	List(opts pkg_api.ListOptions) (*api.BuildList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.Build, err error)
	BuildExpansion
}

// builds implements BuildInterface
type builds struct {
	client *CoreClient
	ns     string
}

// newBuilds returns a Builds
func newBuilds(c *CoreClient, namespace string) *builds {
	return &builds{
		client: c,
		ns:     namespace,
	}
}

// Create takes the representation of a build and creates it.  Returns the server's representation of the build, and an error, if there is any.
func (c *builds) Create(build *api.Build) (result *api.Build, err error) {
	result = &api.Build{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("builds").
		Body(build).
		Do().
		Into(result)
	return
}

// Update takes the representation of a build and updates it. Returns the server's representation of the build, and an error, if there is any.
func (c *builds) Update(build *api.Build) (result *api.Build, err error) {
	result = &api.Build{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("builds").
		Name(build.Name).
		Body(build).
		Do().
		Into(result)
	return
}

// Delete takes name of the build and deletes it. Returns an error if one occurs.
func (c *builds) Delete(name string, options *pkg_api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("builds").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *builds) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("builds").
		VersionedParams(&listOptions, pkg_api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the build, and returns the corresponding build object, and an error if there is any.
func (c *builds) Get(name string) (result *api.Build, err error) {
	result = &api.Build{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("builds").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Builds that match those selectors.
func (c *builds) List(opts pkg_api.ListOptions) (result *api.BuildList, err error) {
	result = &api.BuildList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("builds").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested builds.
func (c *builds) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("builds").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched build.
func (c *builds) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.Build, err error) {
	result = &api.Build{}
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
