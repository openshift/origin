package fake

import (
	api "github.com/openshift/origin/pkg/image/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeImages implements ImageResourceInterface
type FakeImages struct {
	Fake *FakeImage
}

var imagesResource = unversioned.GroupVersionResource{Group: "image.openshift.io", Version: "", Resource: "images"}

func (c *FakeImages) Create(image *api.Image) (result *api.Image, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootCreateAction(imagesResource, image), &api.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.Image), err
}

func (c *FakeImages) Update(image *api.Image) (result *api.Image, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootUpdateAction(imagesResource, image), &api.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.Image), err
}

func (c *FakeImages) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewRootDeleteAction(imagesResource, name), &api.Image{})
	return err
}

func (c *FakeImages) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewRootDeleteCollectionAction(imagesResource, listOptions)

	_, err := c.Fake.Invokes(action, &api.ImageList{})
	return err
}

func (c *FakeImages) Get(name string) (result *api.Image, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootGetAction(imagesResource, name), &api.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.Image), err
}

func (c *FakeImages) List(opts pkg_api.ListOptions) (result *api.ImageList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootListAction(imagesResource, opts), &api.ImageList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := core.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.ImageList{}
	for _, item := range obj.(*api.ImageList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested images.
func (c *FakeImages) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewRootWatchAction(imagesResource, opts))
}

// Patch applies the patch and returns the patched image.
func (c *FakeImages) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.Image, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootPatchSubresourceAction(imagesResource, name, data, subresources...), &api.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.Image), err
}
