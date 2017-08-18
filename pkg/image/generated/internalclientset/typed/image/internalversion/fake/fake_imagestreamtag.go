package fake

import (
	image "github.com/openshift/origin/pkg/image/apis/image"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeImageStreamTags implements ImageStreamTagInterface
type FakeImageStreamTags struct {
	Fake *FakeImage
	ns   string
}

var imagestreamtagsResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "", Resource: "imagestreamtags"}

var imagestreamtagsKind = schema.GroupVersionKind{Group: "image.openshift.io", Version: "", Kind: "ImageStreamTag"}

// Get takes name of the imageStreamTag, and returns the corresponding imageStreamTag object, and an error if there is any.
func (c *FakeImageStreamTags) Get(name string, options v1.GetOptions) (result *image.ImageStreamTag, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(imagestreamtagsResource, c.ns, name), &image.ImageStreamTag{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image.ImageStreamTag), err
}

// Create takes the representation of a imageStreamTag and creates it.  Returns the server's representation of the imageStreamTag, and an error, if there is any.
func (c *FakeImageStreamTags) Create(imageStreamTag *image.ImageStreamTag) (result *image.ImageStreamTag, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(imagestreamtagsResource, c.ns, imageStreamTag), &image.ImageStreamTag{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image.ImageStreamTag), err
}

// Update takes the representation of a imageStreamTag and updates it. Returns the server's representation of the imageStreamTag, and an error, if there is any.
func (c *FakeImageStreamTags) Update(imageStreamTag *image.ImageStreamTag) (result *image.ImageStreamTag, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(imagestreamtagsResource, c.ns, imageStreamTag), &image.ImageStreamTag{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image.ImageStreamTag), err
}

// Delete takes name of the imageStreamTag and deletes it. Returns an error if one occurs.
func (c *FakeImageStreamTags) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(imagestreamtagsResource, c.ns, name), &image.ImageStreamTag{})

	return err
}
