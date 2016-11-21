package fake

import (
	v1 "github.com/openshift/origin/pkg/route/api/v1"
	api "k8s.io/kubernetes/pkg/api"
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

var routesResource = unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "routes"}

func (c *FakeRoutes) Create(route *v1.Route) (result *v1.Route, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(routesResource, c.ns, route), &v1.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Route), err
}

func (c *FakeRoutes) Update(route *v1.Route) (result *v1.Route, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(routesResource, c.ns, route), &v1.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Route), err
}

func (c *FakeRoutes) UpdateStatus(route *v1.Route) (*v1.Route, error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateSubresourceAction(routesResource, "status", c.ns, route), &v1.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Route), err
}

func (c *FakeRoutes) Delete(name string, options *api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(routesResource, c.ns, name), &v1.Route{})

	return err
}

func (c *FakeRoutes) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	action := core.NewDeleteCollectionAction(routesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.RouteList{})
	return err
}

func (c *FakeRoutes) Get(name string) (result *v1.Route, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(routesResource, c.ns, name), &v1.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Route), err
}

func (c *FakeRoutes) List(opts api.ListOptions) (result *v1.RouteList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(routesResource, c.ns, opts), &v1.RouteList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
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
func (c *FakeRoutes) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(routesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched route.
func (c *FakeRoutes) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Route, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(routesResource, c.ns, name, data, subresources...), &v1.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Route), err
}
