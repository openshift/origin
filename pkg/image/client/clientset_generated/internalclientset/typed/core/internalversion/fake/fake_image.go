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

// FakeImages implements ImageInterface
type FakeImages struct {
	Fake *FakeCore
	ns   string
}

var imagesResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "images"}

func (c *FakeImages) Create(image *api.Image) (result *api.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(imagesResource, c.ns, image), &api.Image{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Image), err
}

func (c *FakeImages) Update(image *api.Image) (result *api.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(imagesResource, c.ns, image), &api.Image{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Image), err
}

func (c *FakeImages) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(imagesResource, c.ns, name), &api.Image{})

	return err
}

func (c *FakeImages) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(imagesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.ImageList{})
	return err
}

func (c *FakeImages) Get(name string, options v1.GetOptions) (result *api.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(imagesResource, c.ns, name), &api.Image{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Image), err
}

func (c *FakeImages) List(opts v1.ListOptions) (result *api.ImageList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(imagesResource, c.ns, opts), &api.ImageList{})

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
		InvokesWatch(testing.NewWatchAction(imagesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched image.
func (c *FakeImages) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.Image, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(imagesResource, c.ns, name, data, subresources...), &api.Image{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Image), err
}
