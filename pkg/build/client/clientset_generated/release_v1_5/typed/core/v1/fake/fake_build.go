package fake

import (
	v1 "github.com/openshift/origin/pkg/build/api/v1"
	api "k8s.io/kubernetes/pkg/api"
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

var buildsResource = unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "builds"}

func (c *FakeBuilds) Create(build *v1.Build) (result *v1.Build, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(buildsResource, c.ns, build), &v1.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Build), err
}

func (c *FakeBuilds) Update(build *v1.Build) (result *v1.Build, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(buildsResource, c.ns, build), &v1.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Build), err
}

func (c *FakeBuilds) UpdateStatus(build *v1.Build) (*v1.Build, error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateSubresourceAction(buildsResource, "status", c.ns, build), &v1.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Build), err
}

func (c *FakeBuilds) Delete(name string, options *api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(buildsResource, c.ns, name), &v1.Build{})

	return err
}

func (c *FakeBuilds) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	action := core.NewDeleteCollectionAction(buildsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.BuildList{})
	return err
}

func (c *FakeBuilds) Get(name string) (result *v1.Build, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(buildsResource, c.ns, name), &v1.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Build), err
}

func (c *FakeBuilds) List(opts api.ListOptions) (result *v1.BuildList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(buildsResource, c.ns, opts), &v1.BuildList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.BuildList{}
	for _, item := range obj.(*v1.BuildList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested builds.
func (c *FakeBuilds) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(buildsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched build.
func (c *FakeBuilds) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Build, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(buildsResource, c.ns, name, data, subresources...), &v1.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Build), err
}
