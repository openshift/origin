package fake

import (
	image_v1 "github.com/openshift/api/image/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeImageStreamTags implements ImageStreamTagInterface
type FakeImageStreamTags struct {
	Fake *FakeImageV1
	ns   string
}

var imagestreamtagsResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "imagestreamtags"}

var imagestreamtagsKind = schema.GroupVersionKind{Group: "image.openshift.io", Version: "v1", Kind: "ImageStreamTag"}

// Get takes name of the imageStreamTag, and returns the corresponding imageStreamTag object, and an error if there is any.
func (c *FakeImageStreamTags) Get(name string, options v1.GetOptions) (result *image_v1.ImageStreamTag, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(imagestreamtagsResource, c.ns, name), &image_v1.ImageStreamTag{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image_v1.ImageStreamTag), err
}

// Create takes the representation of a imageStreamTag and creates it.  Returns the server's representation of the imageStreamTag, and an error, if there is any.
func (c *FakeImageStreamTags) Create(imageStreamTag *image_v1.ImageStreamTag) (result *image_v1.ImageStreamTag, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(imagestreamtagsResource, c.ns, imageStreamTag), &image_v1.ImageStreamTag{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image_v1.ImageStreamTag), err
}

// Update takes the representation of a imageStreamTag and updates it. Returns the server's representation of the imageStreamTag, and an error, if there is any.
func (c *FakeImageStreamTags) Update(imageStreamTag *image_v1.ImageStreamTag) (result *image_v1.ImageStreamTag, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(imagestreamtagsResource, c.ns, imageStreamTag), &image_v1.ImageStreamTag{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image_v1.ImageStreamTag), err
}

// Delete takes name of the imageStreamTag and deletes it. Returns an error if one occurs.
func (c *FakeImageStreamTags) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(imagestreamtagsResource, c.ns, name), &image_v1.ImageStreamTag{})

	return err
}
