package fake

import (
	image "github.com/openshift/origin/pkg/image/apis/image"
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

var imagesKind = schema.GroupVersionKind{Group: "image.openshift.io", Version: "", Kind: "Image"}

func (c *FakeImages) Create(imageResource *image.Image) (result *image.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(imagesResource, imageResource), &image.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*image.Image), err
}

func (c *FakeImages) Update(imageResource *image.Image) (result *image.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(imagesResource, imageResource), &image.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*image.Image), err
}

func (c *FakeImages) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(imagesResource, name), &image.Image{})
	return err
}

func (c *FakeImages) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(imagesResource, listOptions)

	_, err := c.Fake.Invokes(action, &image.ImageList{})
	return err
}

func (c *FakeImages) Get(name string, options v1.GetOptions) (result *image.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(imagesResource, name), &image.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*image.Image), err
}

func (c *FakeImages) List(opts v1.ListOptions) (result *image.ImageList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(imagesResource, imagesKind, opts), &image.ImageList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &image.ImageList{}
	for _, item := range obj.(*image.ImageList).Items {
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

// Patch applies the patch and returns the patched imageResource.
func (c *FakeImages) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *image.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(imagesResource, name, data, subresources...), &image.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*image.Image), err
}
