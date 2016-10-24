package fake

import (
	api "github.com/openshift/origin/pkg/route/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeRoutes implements RouteInterface
type FakeRoutes struct {
	Fake *FakeCore
	ns   string
}

var routesResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "routes"}

func (c *FakeRoutes) Create(route *api.Route) (result *api.Route, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(routesResource, c.ns, route), &api.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Route), err
}

func (c *FakeRoutes) Update(route *api.Route) (result *api.Route, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(routesResource, c.ns, route), &api.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Route), err
}

func (c *FakeRoutes) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(routesResource, c.ns, name), &api.Route{})

	return err
}

func (c *FakeRoutes) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewDeleteCollectionAction(routesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.RouteList{})
	return err
}

func (c *FakeRoutes) Get(name string) (result *api.Route, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(routesResource, c.ns, name), &api.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Route), err
}

func (c *FakeRoutes) List(opts pkg_api.ListOptions) (result *api.RouteList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(routesResource, c.ns, opts), &api.RouteList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
	if label == nil {
		label = labels.Everything()
	}
	list := &api.RouteList{}
	for _, item := range obj.(*api.RouteList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested routes.
func (c *FakeRoutes) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(routesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched route.
func (c *FakeRoutes) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.Route, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(routesResource, c.ns, name, data, subresources...), &api.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Route), err
}
