package client

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

// RoutesNamespacer has methods to work with Route resources in a namespace
type RoutesNamespacer interface {
	Routes(namespace string) RouteInterface
}

// RouteInterface exposes methods on Route resources
type RouteInterface interface {
	List(opts metav1.ListOptions) (*routeapi.RouteList, error)
	Get(name string, options metav1.GetOptions) (*routeapi.Route, error)
	Create(route *routeapi.Route) (*routeapi.Route, error)
	Update(route *routeapi.Route) (*routeapi.Route, error)
	UpdateStatus(route *routeapi.Route) (*routeapi.Route, error)
	Delete(name string) error
	Watch(opts metav1.ListOptions) (watch.Interface, error)
}

// routes implements RouteInterface interface
type routes struct {
	r  *Client
	ns string
}

// newRoutes returns a routes
func newRoutes(c *Client, namespace string) *routes {
	return &routes{
		r:  c,
		ns: namespace,
	}
}

// List takes a label and field selector, and returns the list of routes that match that selectors
func (c *routes) List(opts metav1.ListOptions) (result *routeapi.RouteList, err error) {
	result = &routeapi.RouteList{}
	err = c.r.Get().
		Namespace(c.ns).
		Resource("routes").
		VersionedParams(&opts, kapi.ParameterCodec).
		Do().
		Into(result)
	return
}

// Get takes the name of the route, and returns the corresponding Route object, and an error if it occurs
func (c *routes) Get(name string, options metav1.GetOptions) (result *routeapi.Route, err error) {
	result = &routeapi.Route{}
	err = c.r.Get().Namespace(c.ns).Resource("routes").Name(name).VersionedParams(&options, kapi.ParameterCodec).Do().Into(result)
	return
}

// Delete takes the name of the route, and returns an error if one occurs
func (c *routes) Delete(name string) error {
	return c.r.Delete().Namespace(c.ns).Resource("routes").Name(name).Do().Error()
}

// Create takes the representation of a route.  Returns the server's representation of the route, and an error, if it occurs
func (c *routes) Create(route *routeapi.Route) (result *routeapi.Route, err error) {
	result = &routeapi.Route{}
	err = c.r.Post().Namespace(c.ns).Resource("routes").Body(route).Do().Into(result)
	return
}

// Update takes the representation of a route to update.  Returns the server's representation of the route, and an error, if it occurs
func (c *routes) Update(route *routeapi.Route) (result *routeapi.Route, err error) {
	result = &routeapi.Route{}
	err = c.r.Put().Namespace(c.ns).Resource("routes").Name(route.Name).Body(route).Do().Into(result)
	return
}

// UpdateStatus takes the route with altered status.  Returns the server's representation of the route, and an error, if it occurs.
func (c *routes) UpdateStatus(route *routeapi.Route) (result *routeapi.Route, err error) {
	result = &routeapi.Route{}
	err = c.r.Put().Namespace(c.ns).Resource("routes").Name(route.Name).SubResource("status").Body(route).Do().Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested routes.
func (c *routes) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.r.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("routes").
		VersionedParams(&opts, kapi.ParameterCodec).
		Watch()
}
