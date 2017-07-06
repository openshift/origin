package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

// FakeRoutes implements RouteInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeRoutes struct {
	Fake      *Fake
	Namespace string
}

var routesResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "routes"}
var routesKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "Route"}

func (c *FakeRoutes) Get(name string, options metav1.GetOptions) (*routeapi.Route, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(routesResource, c.Namespace, name), &routeapi.Route{})
	if obj == nil {
		return nil, err
	}

	return obj.(*routeapi.Route), err
}

func (c *FakeRoutes) List(opts metav1.ListOptions) (*routeapi.RouteList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewListAction(routesResource, routesKind, c.Namespace, opts), &routeapi.RouteList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*routeapi.RouteList), err
}

func (c *FakeRoutes) Create(inObj *routeapi.Route) (*routeapi.Route, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(routesResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*routeapi.Route), err
}

func (c *FakeRoutes) Update(inObj *routeapi.Route) (*routeapi.Route, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewUpdateAction(routesResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*routeapi.Route), err
}

func (c *FakeRoutes) UpdateStatus(inObj *routeapi.Route) (*routeapi.Route, error) {
	action := clientgotesting.NewUpdateAction(routesResource, c.Namespace, inObj)
	action.Subresource = "status"
	obj, err := c.Fake.Invokes(action, inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*routeapi.Route), err
}

func (c *FakeRoutes) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewDeleteAction(routesResource, c.Namespace, name), &routeapi.Route{})
	return err
}

func (c *FakeRoutes) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewWatchAction(routesResource, c.Namespace, opts))
}
