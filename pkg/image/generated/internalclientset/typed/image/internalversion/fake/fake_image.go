package fake

import (
	api "github.com/openshift/origin/pkg/image/api"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeImages implements ImageResourceInterface
type FakeImages struct {
	Fake *FakeImage
}

var imagesResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "", Resource: "images"}

func (c *FakeImages) Create(image *api.Image) (result *api.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(imagesResource, image), &api.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.Image), err
}

func (c *FakeImages) Update(image *api.Image) (result *api.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(imagesResource, image), &api.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.Image), err
}

func (c *FakeImages) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(imagesResource, name), &api.Image{})
	return err
}

func (c *FakeImages) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(imagesResource, listOptions)

	_, err := c.Fake.Invokes(action, &api.ImageList{})
	return err
}

func (c *FakeImages) Get(name string, options v1.GetOptions) (result *api.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(imagesResource, name), &api.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.Image), err
}

func (c *FakeImages) List(opts v1.ListOptions) (result *api.ImageList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(imagesResource, opts), &api.ImageList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
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
func (c *FakeImages) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(imagesResource, opts))
}

// Patch applies the patch and returns the patched image.
func (c *FakeImages) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(imagesResource, name, data, subresources...), &api.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.Image), err
}
