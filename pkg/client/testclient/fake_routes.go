package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/testclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// FakeRoutes implements RouteInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeRoutes struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeRoutes) Get(name string) (*routeapi.Route, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewGetAction("routes", c.Namespace, name), &routeapi.Route{})
	if obj == nil {
		return nil, err
	}

	return obj.(*routeapi.Route), err
}

func (c *FakeRoutes) List(label labels.Selector, field fields.Selector) (*routeapi.RouteList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewListAction("routes", c.Namespace, label, field), &routeapi.RouteList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*routeapi.RouteList), err
}

func (c *FakeRoutes) Create(inObj *routeapi.Route) (*routeapi.Route, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewCreateAction("routes", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*routeapi.Route), err
}

func (c *FakeRoutes) Update(inObj *routeapi.Route) (*routeapi.Route, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewUpdateAction("routes", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*routeapi.Route), err
}

func (c *FakeRoutes) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewDeleteAction("routes", c.Namespace, name), &routeapi.Route{})
	return err
}

func (c *FakeRoutes) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Invokes(ktestclient.NewWatchAction("routes", c.Namespace, label, field, resourceVersion), nil)
	return c.Fake.Watch, nil
}
