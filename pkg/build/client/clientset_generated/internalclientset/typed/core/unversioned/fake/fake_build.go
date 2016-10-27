package fake

import (
	api "github.com/openshift/origin/pkg/build/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeBuilds implements BuildInterface
type FakeBuilds struct {
	Fake *FakeCore
	ns   string
}

var buildsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "builds"}

func (c *FakeBuilds) Create(build *api.Build) (result *api.Build, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(buildsResource, c.ns, build), &api.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Build), err
}

func (c *FakeBuilds) Update(build *api.Build) (result *api.Build, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(buildsResource, c.ns, build), &api.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Build), err
}

func (c *FakeBuilds) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(buildsResource, c.ns, name), &api.Build{})

	return err
}

func (c *FakeBuilds) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewDeleteCollectionAction(buildsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.BuildList{})
	return err
}

func (c *FakeBuilds) Get(name string) (result *api.Build, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(buildsResource, c.ns, name), &api.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Build), err
}

func (c *FakeBuilds) List(opts pkg_api.ListOptions) (result *api.BuildList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(buildsResource, c.ns, opts), &api.BuildList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
	if label == nil {
		label = labels.Everything()
	}
	list := &api.BuildList{}
	for _, item := range obj.(*api.BuildList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested builds.
func (c *FakeBuilds) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(buildsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched build.
func (c *FakeBuilds) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.Build, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(buildsResource, c.ns, name, data, subresources...), &api.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Build), err
}
