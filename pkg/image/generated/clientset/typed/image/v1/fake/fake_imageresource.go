package fake

import (
	v1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeImages implements ImageResourceInterface
type FakeImages struct {
	Fake *FakeImageV1
}

var imagesResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "images"}

var imagesKind = schema.GroupVersionKind{Group: "image.openshift.io", Version: "v1", Kind: "Image"}

func (c *FakeImages) Create(imageResource *v1.Image) (result *v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(imagesResource, imageResource), &v1.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Image), err
}

func (c *FakeImages) Update(imageResource *v1.Image) (result *v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(imagesResource, imageResource), &v1.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Image), err
}

func (c *FakeImages) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(imagesResource, name), &v1.Image{})
	return err
}

func (c *FakeImages) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(imagesResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ImageList{})
	return err
}

func (c *FakeImages) Get(name string, options meta_v1.GetOptions) (result *v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(imagesResource, name), &v1.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Image), err
}

func (c *FakeImages) List(opts meta_v1.ListOptions) (result *v1.ImageList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(imagesResource, imagesKind, opts), &v1.ImageList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
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
func (c *FakeImages) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(imagesResource, opts))
}

// Patch applies the patch and returns the patched imageResource.
func (c *FakeImages) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(imagesResource, name, data, subresources...), &v1.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Image), err
}
