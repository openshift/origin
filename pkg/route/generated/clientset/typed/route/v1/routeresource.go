package v1

import (
	v1 "github.com/openshift/origin/pkg/route/apis/route/v1"
	scheme "github.com/openshift/origin/pkg/route/generated/clientset/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// RoutesGetter has a method to return a RouteResourceInterface.
// A group's client should implement this interface.
type RoutesGetter interface {
	Routes(namespace string) RouteResourceInterface
}

// RouteResourceInterface has methods to work with RouteResource resources.
type RouteResourceInterface interface {
	Create(*v1.Route) (*v1.Route, error)
	Update(*v1.Route) (*v1.Route, error)
	UpdateStatus(*v1.Route) (*v1.Route, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.Route, error)
	List(opts meta_v1.ListOptions) (*v1.RouteList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Route, err error)
	RouteResourceExpansion
}

// routes implements RouteResourceInterface
type routes struct {
	client rest.Interface
	ns     string
}

// newRoutes returns a Routes
func newRoutes(c *RouteV1Client, namespace string) *routes {
	return &routes{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the routeResource, and returns the corresponding routeResource object, and an error if there is any.
func (c *routes) Get(name string, options meta_v1.GetOptions) (result *v1.Route, err error) {
	result = &v1.Route{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("routes").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Routes that match those selectors.
func (c *routes) List(opts meta_v1.ListOptions) (result *v1.RouteList, err error) {
	result = &v1.RouteList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("routes").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested routes.
func (c *routes) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("routes").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a routeResource and creates it.  Returns the server's representation of the routeResource, and an error, if there is any.
func (c *routes) Create(routeResource *v1.Route) (result *v1.Route, err error) {
	result = &v1.Route{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("routes").
		Body(routeResource).
		Do().
		Into(result)
	return
}

// Update takes the representation of a routeResource and updates it. Returns the server's representation of the routeResource, and an error, if there is any.
func (c *routes) Update(routeResource *v1.Route) (result *v1.Route, err error) {
	result = &v1.Route{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("routes").
		Name(routeResource.Name).
		Body(routeResource).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *routes) UpdateStatus(routeResource *v1.Route) (result *v1.Route, err error) {
	result = &v1.Route{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("routes").
		Name(routeResource.Name).
		SubResource("status").
		Body(routeResource).
		Do().
		Into(result)
	return
}

// Delete takes name of the routeResource and deletes it. Returns an error if one occurs.
func (c *routes) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("routes").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *routes) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("routes").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched routeResource.
func (c *routes) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Route, err error) {
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
