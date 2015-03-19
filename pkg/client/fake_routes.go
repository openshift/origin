package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// FakeRoutes implements BuildInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeRoutes struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeRoutes) List(label labels.Selector, field fields.Selector) (*routeapi.RouteList, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "list-routes"})
	return &routeapi.RouteList{}, nil
}

func (c *FakeRoutes) Get(name string) (*routeapi.Route, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-route"})
	return &routeapi.Route{}, nil
}

func (c *FakeRoutes) Create(route *routeapi.Route) (*routeapi.Route, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-route"})
	return &routeapi.Route{}, nil
}

func (c *FakeRoutes) Update(route *routeapi.Route) (*routeapi.Route, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "update-route"})
	return &routeapi.Route{}, nil
}

func (c *FakeRoutes) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-route"})
	return nil
}

func (c *FakeRoutes) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-routes"})
	return nil, nil
}
