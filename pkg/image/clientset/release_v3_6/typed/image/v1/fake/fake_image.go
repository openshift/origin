package fake

import (
	v1 "github.com/openshift/origin/pkg/image/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	api_v1 "k8s.io/kubernetes/pkg/api/v1"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeImages implements ImageResourceInterface
type FakeImages struct {
	Fake *FakeImageV1
}

var imagesResource = unversioned.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "images"}

func (c *FakeImages) Create(image *v1.Image) (result *v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootCreateAction(imagesResource, image), &v1.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Image), err
}

func (c *FakeImages) Update(image *v1.Image) (result *v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootUpdateAction(imagesResource, image), &v1.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Image), err
}

func (c *FakeImages) Delete(name string, options *api_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewRootDeleteAction(imagesResource, name), &v1.Image{})
	return err
}

func (c *FakeImages) DeleteCollection(options *api_v1.DeleteOptions, listOptions api_v1.ListOptions) error {
	action := core.NewRootDeleteCollectionAction(imagesResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ImageList{})
	return err
}

func (c *FakeImages) Get(name string) (result *v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootGetAction(imagesResource, name), &v1.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Image), err
}

func (c *FakeImages) List(opts api_v1.ListOptions) (result *v1.ImageList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootListAction(imagesResource, opts), &v1.ImageList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := core.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.ImageList{}
	for _, item := range obj.(*v1.ImageList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested images.
func (c *FakeImages) Watch(opts api_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewRootWatchAction(imagesResource, opts))
}

// Patch applies the patch and returns the patched image.
func (c *FakeImages) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootPatchSubresourceAction(imagesResource, name, data, subresources...), &v1.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Image), err
}
