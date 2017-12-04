package fake

import (
	image "github.com/openshift/origin/pkg/image/apis/image"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
	core "k8s.io/kubernetes/pkg/apis/core"
)

// FakeImageStreams implements ImageStreamInterface
type FakeImageStreams struct {
	Fake *FakeImage
	ns   string
}

var imagestreamsResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "", Resource: "imagestreams"}

var imagestreamsKind = schema.GroupVersionKind{Group: "image.openshift.io", Version: "", Kind: "ImageStream"}

// Get takes name of the imageStream, and returns the corresponding imageStream object, and an error if there is any.
func (c *FakeImageStreams) Get(name string, options v1.GetOptions) (result *image.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(imagestreamsResource, c.ns, name), &image.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image.ImageStream), err
}

// List takes label and field selectors, and returns the list of ImageStreams that match those selectors.
func (c *FakeImageStreams) List(opts v1.ListOptions) (result *image.ImageStreamList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(imagestreamsResource, imagestreamsKind, c.ns, opts), &image.ImageStreamList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &image.ImageStreamList{}
	for _, item := range obj.(*image.ImageStreamList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested imageStreams.
func (c *FakeImageStreams) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(imagestreamsResource, c.ns, opts))

}

// Create takes the representation of a imageStream and creates it.  Returns the server's representation of the imageStream, and an error, if there is any.
func (c *FakeImageStreams) Create(imageStream *image.ImageStream) (result *image.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(imagestreamsResource, c.ns, imageStream), &image.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image.ImageStream), err
}

// Update takes the representation of a imageStream and updates it. Returns the server's representation of the imageStream, and an error, if there is any.
func (c *FakeImageStreams) Update(imageStream *image.ImageStream) (result *image.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(imagestreamsResource, c.ns, imageStream), &image.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image.ImageStream), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeImageStreams) UpdateStatus(imageStream *image.ImageStream) (*image.ImageStream, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(imagestreamsResource, "status", c.ns, imageStream), &image.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image.ImageStream), err
}

// Delete takes name of the imageStream and deletes it. Returns an error if one occurs.
func (c *FakeImageStreams) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(imagestreamsResource, c.ns, name), &image.ImageStream{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeImageStreams) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(imagestreamsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &image.ImageStreamList{})
	return err
}

// Patch applies the patch and returns the patched imageStream.
func (c *FakeImageStreams) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *image.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(imagestreamsResource, c.ns, name, data, subresources...), &image.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image.ImageStream), err
}

// Secrets takes label and field selectors, and returns the list of Secrets that match those selectors.
func (c *FakeImageStreams) Secrets(imageStreamName string, opts v1.ListOptions) (result *core.SecretList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListSubresourceAction(imagestreamsResource, imageStreamName, "secrets", imagestreamsKind, c.ns, opts), &core.SecretList{})

	if obj == nil {
		return nil, err
	}
	return obj.(*core.SecretList), err
}
