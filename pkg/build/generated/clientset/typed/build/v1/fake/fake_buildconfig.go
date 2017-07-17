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

// FakeBuildConfigs implements BuildConfigInterface
type FakeBuildConfigs struct {
	Fake *FakeBuildV1
	ns   string
}

var buildconfigsResource = schema.GroupVersionResource{Group: "build.openshift.io", Version: "v1", Resource: "buildconfigs"}

var buildconfigsKind = schema.GroupVersionKind{Group: "build.openshift.io", Version: "v1", Kind: "BuildConfig"}

func (c *FakeBuildConfigs) Create(buildConfig *v1.BuildConfig) (result *v1.BuildConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(buildconfigsResource, c.ns, buildConfig), &v1.BuildConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.BuildConfig), err
}

func (c *FakeBuildConfigs) Update(buildConfig *v1.BuildConfig) (result *v1.BuildConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(buildconfigsResource, c.ns, buildConfig), &v1.BuildConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.BuildConfig), err
}

func (c *FakeBuildConfigs) UpdateStatus(buildConfig *v1.BuildConfig) (*v1.BuildConfig, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(buildconfigsResource, "status", c.ns, buildConfig), &v1.BuildConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.BuildConfig), err
}

func (c *FakeBuildConfigs) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(buildconfigsResource, c.ns, name), &v1.BuildConfig{})

	return err
}

func (c *FakeBuildConfigs) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(buildconfigsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.BuildConfigList{})
	return err
}

func (c *FakeBuildConfigs) Get(name string, options meta_v1.GetOptions) (result *v1.BuildConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(buildconfigsResource, c.ns, name), &v1.BuildConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.BuildConfig), err
}

func (c *FakeBuildConfigs) List(opts meta_v1.ListOptions) (result *v1.BuildConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(buildconfigsResource, buildconfigsKind, c.ns, opts), &v1.BuildConfigList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.BuildConfigList{}
	for _, item := range obj.(*v1.BuildConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested buildConfigs.
func (c *FakeBuildConfigs) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(buildconfigsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched buildConfig.
func (c *FakeBuildConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.BuildConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(buildconfigsResource, c.ns, name, data, subresources...), &v1.BuildConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.BuildConfig), err
}
