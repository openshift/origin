package internalversion

import (
	api "github.com/openshift/origin/pkg/route/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	watch "k8s.io/kubernetes/pkg/watch"
)

// RoutesGetter has a method to return a RouteResourceInterface.
// A group's client should implement this interface.
type RoutesGetter interface {
	Routes(namespace string) RouteResourceInterface
}

// RouteResourceInterface has methods to work with RouteResource resources.
type RouteResourceInterface interface {
	Create(*api.Route) (*api.Route, error)
	Update(*api.Route) (*api.Route, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.Route, error)
	List(opts pkg_api.ListOptions) (*api.RouteList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.Route, err error)
	RouteResourceExpansion
}

// routes implements RouteResourceInterface
type routes struct {
	client restclient.Interface
	ns     string
}

// newRoutes returns a Routes
func newRoutes(c *RouteClient, namespace string) *routes {
	return &routes{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a route and creates it.  Returns the server's representation of the route, and an error, if there is any.
func (c *routes) Create(route *api.Route) (result *api.Route, err error) {
	result = &api.Route{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("routes").
		Body(route).
		Do().
		Into(result)
	return
}

// Update takes the representation of a route and updates it. Returns the server's representation of the route, and an error, if there is any.
func (c *routes) Update(route *api.Route) (result *api.Route, err error) {
	result = &api.Route{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("routes").
		Name(route.Name).
		Body(route).
		Do().
		Into(result)
	return
}

// Delete takes name of the route and deletes it. Returns an error if one occurs.
func (c *routes) Delete(name string, options *pkg_api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("routes").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *routes) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("routes").
		VersionedParams(&listOptions, pkg_api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the route, and returns the corresponding route object, and an error if there is any.
func (c *routes) Get(name string) (result *api.Route, err error) {
	result = &api.Route{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("routes").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Routes that match those selectors.
func (c *routes) List(opts pkg_api.ListOptions) (result *api.RouteList, err error) {
	result = &api.RouteList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("routes").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested routes.
func (c *routes) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("routes").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched route.
func (c *routes) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.Route, err error) {
	result = &api.Route{}
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
