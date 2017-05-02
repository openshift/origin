package fake

import (
	api "github.com/openshift/origin/pkg/image/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeImageStreamTags implements ImageStreamTagInterface
type FakeImageStreamTags struct {
	Fake *FakeImage
	ns   string
}

var imagestreamtagsResource = unversioned.GroupVersionResource{Group: "image.openshift.io", Version: "", Resource: "imagestreamtags"}

func (c *FakeImageStreamTags) Create(imageStreamTag *api.ImageStreamTag) (result *api.ImageStreamTag, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(imagestreamtagsResource, c.ns, imageStreamTag), &api.ImageStreamTag{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStreamTag), err
}

func (c *FakeImageStreamTags) Update(imageStreamTag *api.ImageStreamTag) (result *api.ImageStreamTag, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(imagestreamtagsResource, c.ns, imageStreamTag), &api.ImageStreamTag{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStreamTag), err
}

func (c *FakeImageStreamTags) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(imagestreamtagsResource, c.ns, name), &api.ImageStreamTag{})

	return err
}

func (c *FakeImageStreamTags) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewDeleteCollectionAction(imagestreamtagsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.ImageStreamTagList{})
	return err
}

func (c *FakeImageStreamTags) Get(name string) (result *api.ImageStreamTag, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(imagestreamtagsResource, c.ns, name), &api.ImageStreamTag{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStreamTag), err
}

func (c *FakeImageStreamTags) List(opts pkg_api.ListOptions) (result *api.ImageStreamTagList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(imagestreamtagsResource, c.ns, opts), &api.ImageStreamTagList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := core.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.ImageStreamTagList{}
	for _, item := range obj.(*api.ImageStreamTagList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested imageStreamTags.
func (c *FakeImageStreamTags) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(imagestreamtagsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched imageStreamTag.
func (c *FakeImageStreamTags) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ImageStreamTag, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(imagestreamtagsResource, c.ns, name, data, subresources...), &api.ImageStreamTag{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStreamTag), err
}
