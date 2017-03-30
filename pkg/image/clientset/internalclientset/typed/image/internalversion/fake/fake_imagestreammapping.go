package fake

import (
	api "github.com/openshift/origin/pkg/image/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeImageStreamMappings implements ImageStreamMappingInterface
type FakeImageStreamMappings struct {
	Fake *FakeImage
	ns   string
}

var imagestreammappingsResource = unversioned.GroupVersionResource{Group: "image.openshift.io", Version: "", Resource: "imagestreammappings"}

func (c *FakeImageStreamMappings) Create(imageStreamMapping *api.ImageStreamMapping) (result *api.ImageStreamMapping, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(imagestreammappingsResource, c.ns, imageStreamMapping), &api.ImageStreamMapping{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStreamMapping), err
}

func (c *FakeImageStreamMappings) Update(imageStreamMapping *api.ImageStreamMapping) (result *api.ImageStreamMapping, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(imagestreammappingsResource, c.ns, imageStreamMapping), &api.ImageStreamMapping{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStreamMapping), err
}

func (c *FakeImageStreamMappings) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(imagestreammappingsResource, c.ns, name), &api.ImageStreamMapping{})

	return err
}

func (c *FakeImageStreamMappings) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewDeleteCollectionAction(imagestreammappingsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.ImageStreamMappingList{})
	return err
}

func (c *FakeImageStreamMappings) Get(name string) (result *api.ImageStreamMapping, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(imagestreammappingsResource, c.ns, name), &api.ImageStreamMapping{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStreamMapping), err
}

func (c *FakeImageStreamMappings) List(opts pkg_api.ListOptions) (result *api.ImageStreamMappingList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(imagestreammappingsResource, c.ns, opts), &api.ImageStreamMappingList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := core.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.ImageStreamMappingList{}
	for _, item := range obj.(*api.ImageStreamMappingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested imageStreamMappings.
func (c *FakeImageStreamMappings) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(imagestreammappingsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched imageStreamMapping.
func (c *FakeImageStreamMappings) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ImageStreamMapping, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(imagestreammappingsResource, c.ns, name, data, subresources...), &api.ImageStreamMapping{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStreamMapping), err
}
