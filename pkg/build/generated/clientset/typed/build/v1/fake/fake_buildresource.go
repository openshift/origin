package fake

import (
	v1 "github.com/openshift/origin/pkg/build/apis/build/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeBuilds implements BuildResourceInterface
type FakeBuilds struct {
	Fake *FakeBuildV1
	ns   string
}

var buildsResource = schema.GroupVersionResource{Group: "build.openshift.io", Version: "v1", Resource: "builds"}

var buildsKind = schema.GroupVersionKind{Group: "build.openshift.io", Version: "v1", Kind: "Build"}

func (c *FakeBuilds) Create(buildResource *v1.Build) (result *v1.Build, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(buildsResource, c.ns, buildResource), &v1.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Build), err
}

func (c *FakeBuilds) Update(buildResource *v1.Build) (result *v1.Build, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(buildsResource, c.ns, buildResource), &v1.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Build), err
}

func (c *FakeBuilds) UpdateStatus(buildResource *v1.Build) (*v1.Build, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(buildsResource, "status", c.ns, buildResource), &v1.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Build), err
}

func (c *FakeBuilds) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(buildsResource, c.ns, name), &v1.Build{})

	return err
}

func (c *FakeBuilds) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(buildsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.BuildList{})
	return err
}

func (c *FakeBuilds) Get(name string, options meta_v1.GetOptions) (result *v1.Build, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(buildsResource, c.ns, name), &v1.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Build), err
}

func (c *FakeBuilds) List(opts meta_v1.ListOptions) (result *v1.BuildList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(buildsResource, buildsKind, c.ns, opts), &v1.BuildList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
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
func (c *FakeBuilds) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(buildsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched buildResource.
func (c *FakeBuilds) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Build, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(buildsResource, c.ns, name, data, subresources...), &v1.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Build), err
}
