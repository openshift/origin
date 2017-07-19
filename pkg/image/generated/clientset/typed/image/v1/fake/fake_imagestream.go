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

// FakeImageStreams implements ImageStreamInterface
type FakeImageStreams struct {
	Fake *FakeImageV1
	ns   string
}

var imagestreamsResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "imagestreams"}

var imagestreamsKind = schema.GroupVersionKind{Group: "image.openshift.io", Version: "v1", Kind: "ImageStream"}

func (c *FakeImageStreams) Create(imageStream *v1.ImageStream) (result *v1.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(imagestreamsResource, c.ns, imageStream), &v1.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ImageStream), err
}

func (c *FakeImageStreams) Update(imageStream *v1.ImageStream) (result *v1.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(imagestreamsResource, c.ns, imageStream), &v1.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ImageStream), err
}

func (c *FakeImageStreams) UpdateStatus(imageStream *v1.ImageStream) (*v1.ImageStream, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(imagestreamsResource, "status", c.ns, imageStream), &v1.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ImageStream), err
}

func (c *FakeImageStreams) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(imagestreamsResource, c.ns, name), &v1.ImageStream{})

	return err
}

func (c *FakeImageStreams) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(imagestreamsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ImageStreamList{})
	return err
}

func (c *FakeImageStreams) Get(name string, options meta_v1.GetOptions) (result *v1.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(imagestreamsResource, c.ns, name), &v1.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ImageStream), err
}

func (c *FakeImageStreams) List(opts meta_v1.ListOptions) (result *v1.ImageStreamList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(imagestreamsResource, imagestreamsKind, c.ns, opts), &v1.ImageStreamList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.ImageStreamList{}
	for _, item := range obj.(*v1.ImageStreamList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested imageStreams.
func (c *FakeImageStreams) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(imagestreamsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched imageStream.
func (c *FakeImageStreams) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(imagestreamsResource, c.ns, name, data, subresources...), &v1.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ImageStream), err
}
