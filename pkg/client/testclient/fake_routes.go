package testclient

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// FakeRoutes implements RouteInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeRoutes struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeRoutes) List(label labels.Selector, field fields.Selector) (*routeapi.RouteList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-routes"}, &routeapi.RouteList{})
	return obj.(*routeapi.RouteList), err
}

func (c *FakeRoutes) Get(name string) (*routeapi.Route, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-route"}, &routeapi.Route{})
	return obj.(*routeapi.Route), err
}

func (c *FakeRoutes) Create(route *routeapi.Route) (*routeapi.Route, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-route"}, &routeapi.Route{})
	return obj.(*routeapi.Route), err
}

func (c *FakeRoutes) Update(route *routeapi.Route) (*routeapi.Route, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "update-route"}, &routeapi.Route{})
	return obj.(*routeapi.Route), err
}

func (c *FakeRoutes) Delete(name string) error {
	c.Fake.Lock.Lock()
	defer c.Fake.Lock.Unlock()

	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-route"})
	return nil
}

func (c *FakeRoutes) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Lock.Lock()
	defer c.Fake.Lock.Unlock()

	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-routes"})
	return nil, nil
}
