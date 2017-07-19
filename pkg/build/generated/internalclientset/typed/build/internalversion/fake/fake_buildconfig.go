package fake

import (
	build "github.com/openshift/origin/pkg/build/apis/build"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeBuildConfigs implements BuildConfigInterface
type FakeBuildConfigs struct {
	Fake *FakeBuild
	ns   string
}

var buildconfigsResource = schema.GroupVersionResource{Group: "build.openshift.io", Version: "", Resource: "buildconfigs"}

var buildconfigsKind = schema.GroupVersionKind{Group: "build.openshift.io", Version: "", Kind: "BuildConfig"}

func (c *FakeBuildConfigs) Create(buildConfig *build.BuildConfig) (result *build.BuildConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(buildconfigsResource, c.ns, buildConfig), &build.BuildConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*build.BuildConfig), err
}

func (c *FakeBuildConfigs) Update(buildConfig *build.BuildConfig) (result *build.BuildConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(buildconfigsResource, c.ns, buildConfig), &build.BuildConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*build.BuildConfig), err
}

func (c *FakeBuildConfigs) UpdateStatus(buildConfig *build.BuildConfig) (*build.BuildConfig, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(buildconfigsResource, "status", c.ns, buildConfig), &build.BuildConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*build.BuildConfig), err
}

func (c *FakeBuildConfigs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(buildconfigsResource, c.ns, name), &build.BuildConfig{})

	return err
}

func (c *FakeBuildConfigs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(buildconfigsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &build.BuildConfigList{})
	return err
}

func (c *FakeBuildConfigs) Get(name string, options v1.GetOptions) (result *build.BuildConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(buildconfigsResource, c.ns, name), &build.BuildConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*build.BuildConfig), err
}

func (c *FakeBuildConfigs) List(opts v1.ListOptions) (result *build.BuildConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(buildconfigsResource, buildconfigsKind, c.ns, opts), &build.BuildConfigList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &build.BuildConfigList{}
	for _, item := range obj.(*build.BuildConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested buildConfigs.
func (c *FakeBuildConfigs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(buildconfigsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched buildConfig.
func (c *FakeBuildConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *build.BuildConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(buildconfigsResource, c.ns, name, data, subresources...), &build.BuildConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*build.BuildConfig), err
}
