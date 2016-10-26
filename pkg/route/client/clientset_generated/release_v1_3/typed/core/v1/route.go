package v1

import (
	v1 "github.com/openshift/origin/pkg/route/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	watch "k8s.io/kubernetes/pkg/watch"
)

// RoutesGetter has a method to return a RouteInterface.
// A group's client should implement this interface.
type RoutesGetter interface {
	Routes(namespace string) RouteInterface
}

// RouteInterface has methods to work with Route resources.
type RouteInterface interface {
	Create(*v1.Route) (*v1.Route, error)
	Update(*v1.Route) (*v1.Route, error)
	UpdateStatus(*v1.Route) (*v1.Route, error)
	Delete(name string, options *api.DeleteOptions) error
	DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error
	Get(name string) (*v1.Route, error)
	List(opts api.ListOptions) (*v1.RouteList, error)
	Watch(opts api.ListOptions) (watch.Interface, error)
	Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Route, err error)
	RouteExpansion
}

// routes implements RouteInterface
type routes struct {
	client *CoreClient
	ns     string
}

// newRoutes returns a Routes
func newRoutes(c *CoreClient, namespace string) *routes {
	return &routes{
		client: c,
		ns:     namespace,
	}
}

// Create takes the representation of a route and creates it.  Returns the server's representation of the route, and an error, if there is any.
func (c *routes) Create(route *v1.Route) (result *v1.Route, err error) {
	result = &v1.Route{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("routes").
		Body(route).
		Do().
		Into(result)
	return
}

// Update takes the representation of a route and updates it. Returns the server's representation of the route, and an error, if there is any.
func (c *routes) Update(route *v1.Route) (result *v1.Route, err error) {
	result = &v1.Route{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("routes").
		Name(route.Name).
		Body(route).
		Do().
		Into(result)
	return
}

func (c *routes) UpdateStatus(route *v1.Route) (result *v1.Route, err error) {
	result = &v1.Route{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("routes").
		Name(route.Name).
		SubResource("status").
		Body(route).
		Do().
		Into(result)
	return
}

// Delete takes name of the route and deletes it. Returns an error if one occurs.
func (c *routes) Delete(name string, options *api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("routes").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *routes) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("routes").
		VersionedParams(&listOptions, api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the route, and returns the corresponding route object, and an error if there is any.
func (c *routes) Get(name string) (result *v1.Route, err error) {
	result = &v1.Route{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("routes").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Routes that match those selectors.
func (c *routes) List(opts api.ListOptions) (result *v1.RouteList, err error) {
	result = &v1.RouteList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("routes").
		VersionedParams(&opts, api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested routes.
func (c *routes) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("routes").
		VersionedParams(&opts, api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched route.
func (c *routes) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Route, err error) {
	result = &v1.Route{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("routes").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
