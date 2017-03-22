package v1

import (
	v1 "github.com/openshift/origin/pkg/build/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	api_v1 "k8s.io/kubernetes/pkg/api/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	watch "k8s.io/kubernetes/pkg/watch"
)

// BuildsGetter has a method to return a BuildResourceInterface.
// A group's client should implement this interface.
type BuildsGetter interface {
	Builds(namespace string) BuildResourceInterface
}

// BuildResourceInterface has methods to work with BuildResource resources.
type BuildResourceInterface interface {
	Create(*v1.Build) (*v1.Build, error)
	Update(*v1.Build) (*v1.Build, error)
	UpdateStatus(*v1.Build) (*v1.Build, error)
	Delete(name string, options *api_v1.DeleteOptions) error
	DeleteCollection(options *api_v1.DeleteOptions, listOptions api_v1.ListOptions) error
	Get(name string) (*v1.Build, error)
	List(opts api_v1.ListOptions) (*v1.BuildList, error)
	Watch(opts api_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Build, err error)
	BuildResourceExpansion
}

// builds implements BuildResourceInterface
type builds struct {
	client restclient.Interface
	ns     string
}

// newBuilds returns a Builds
func newBuilds(c *BuildV1Client, namespace string) *builds {
	return &builds{
		client: c.RESTClient(),
		ns:     namespace,
	}
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
func (c *builds) Delete(name string, options *api_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("builds").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *builds) DeleteCollection(options *api_v1.DeleteOptions, listOptions api_v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("builds").
		VersionedParams(&listOptions, api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the build, and returns the corresponding build object, and an error if there is any.
func (c *builds) Get(name string) (result *v1.Build, err error) {
	result = &v1.Build{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("builds").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Builds that match those selectors.
func (c *builds) List(opts api_v1.ListOptions) (result *v1.BuildList, err error) {
	result = &v1.BuildList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("builds").
		VersionedParams(&opts, api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested builds.
func (c *builds) Watch(opts api_v1.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("builds").
		VersionedParams(&opts, api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched build.
func (c *builds) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Build, err error) {
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
