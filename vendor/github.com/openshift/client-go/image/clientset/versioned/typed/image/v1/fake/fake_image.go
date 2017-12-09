package fake

import (
	image_v1 "github.com/openshift/api/image/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeImages implements ImageInterface
type FakeImages struct {
	Fake *FakeImageV1
}

var imagesResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "images"}

var imagesKind = schema.GroupVersionKind{Group: "image.openshift.io", Version: "v1", Kind: "Image"}

// Get takes name of the image, and returns the corresponding image object, and an error if there is any.
func (c *FakeImages) Get(name string, options v1.GetOptions) (result *image_v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(imagesResource, name), &image_v1.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*image_v1.Image), err
}

// List takes label and field selectors, and returns the list of Images that match those selectors.
func (c *FakeImages) List(opts v1.ListOptions) (result *image_v1.ImageList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(imagesResource, imagesKind, opts), &image_v1.ImageList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &image_v1.ImageList{}
	for _, item := range obj.(*image_v1.ImageList).Items {
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

// Create takes the representation of a image and creates it.  Returns the server's representation of the image, and an error, if there is any.
func (c *FakeImages) Create(image *image_v1.Image) (result *image_v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(imagesResource, image), &image_v1.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*image_v1.Image), err
}

// Update takes the representation of a image and updates it. Returns the server's representation of the image, and an error, if there is any.
func (c *FakeImages) Update(image *image_v1.Image) (result *image_v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(imagesResource, image), &image_v1.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*image_v1.Image), err
}

// Delete takes name of the image and deletes it. Returns an error if one occurs.
func (c *FakeImages) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(imagesResource, name), &image_v1.Image{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeImages) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(imagesResource, listOptions)

	_, err := c.Fake.Invokes(action, &image_v1.ImageList{})
	return err
}

// Patch applies the patch and returns the patched image.
func (c *FakeImages) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *image_v1.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(imagesResource, name, data, subresources...), &image_v1.Image{})
	if obj == nil {
		return nil, err
	}
	return obj.(*image_v1.Image), err
}
