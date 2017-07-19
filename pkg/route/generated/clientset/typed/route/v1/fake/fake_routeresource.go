package fake

import (
	v1 "github.com/openshift/origin/pkg/route/apis/route/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeRoutes implements RouteResourceInterface
type FakeRoutes struct {
	Fake *FakeRouteV1
	ns   string
}

var routesResource = schema.GroupVersionResource{Group: "route.openshift.io", Version: "v1", Resource: "routes"}

var routesKind = schema.GroupVersionKind{Group: "route.openshift.io", Version: "v1", Kind: "Route"}

func (c *FakeRoutes) Create(routeResource *v1.Route) (result *v1.Route, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(routesResource, c.ns, routeResource), &v1.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Route), err
}

func (c *FakeRoutes) Update(routeResource *v1.Route) (result *v1.Route, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(routesResource, c.ns, routeResource), &v1.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Route), err
}

func (c *FakeRoutes) UpdateStatus(routeResource *v1.Route) (*v1.Route, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(routesResource, "status", c.ns, routeResource), &v1.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Route), err
}

func (c *FakeRoutes) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(routesResource, c.ns, name), &v1.Route{})

	return err
}

func (c *FakeRoutes) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(routesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.RouteList{})
	return err
}

func (c *FakeRoutes) Get(name string, options meta_v1.GetOptions) (result *v1.Route, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(routesResource, c.ns, name), &v1.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Route), err
}

func (c *FakeRoutes) List(opts meta_v1.ListOptions) (result *v1.RouteList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(routesResource, routesKind, c.ns, opts), &v1.RouteList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.RouteList{}
	for _, item := range obj.(*v1.RouteList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested routes.
func (c *FakeRoutes) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(routesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched routeResource.
func (c *FakeRoutes) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Route, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(routesResource, c.ns, name, data, subresources...), &v1.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Route), err
}
