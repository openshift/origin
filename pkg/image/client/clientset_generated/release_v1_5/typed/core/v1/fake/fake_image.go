package fake

import (
	v1 "github.com/openshift/origin/pkg/image/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeImages implements ImageInterface
type FakeImages struct {
	Fake *FakeCore
	ns   string
}

var imagesResource = unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "images"}

func (c *FakeImages) Create(image *v1.Image) (result *v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(imagesResource, c.ns, image), &v1.Image{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Image), err
}

func (c *FakeImages) Update(image *v1.Image) (result *v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(imagesResource, c.ns, image), &v1.Image{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Image), err
}

func (c *FakeImages) Delete(name string, options *api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(imagesResource, c.ns, name), &v1.Image{})

	return err
}

func (c *FakeImages) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	action := core.NewDeleteCollectionAction(imagesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ImageList{})
	return err
}

func (c *FakeImages) Get(name string) (result *v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(imagesResource, c.ns, name), &v1.Image{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Image), err
}

func (c *FakeImages) List(opts api.ListOptions) (result *v1.ImageList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(imagesResource, c.ns, opts), &v1.ImageList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
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
func (c *FakeImages) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(imagesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched image.
func (c *FakeImages) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(imagesResource, c.ns, name, data, subresources...), &v1.Image{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Image), err
}
