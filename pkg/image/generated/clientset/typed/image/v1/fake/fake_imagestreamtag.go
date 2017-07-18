package fake

import (
	v1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeImageStreamTags implements ImageStreamTagInterface
type FakeImageStreamTags struct {
	Fake *FakeImageV1
	ns   string
}

var imagestreamtagsResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "imagestreamtags"}

func (c *FakeImageStreamTags) Create(imageStreamTag *v1.ImageStreamTag) (result *v1.ImageStreamTag, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(imagestreamtagsResource, c.ns, imageStreamTag), &v1.ImageStreamTag{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ImageStreamTag), err
}

func (c *FakeImageStreamTags) Update(imageStreamTag *v1.ImageStreamTag) (result *v1.ImageStreamTag, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(imagestreamtagsResource, c.ns, imageStreamTag), &v1.ImageStreamTag{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ImageStreamTag), err
}

func (c *FakeImageStreamTags) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(imagestreamtagsResource, c.ns, name), &v1.ImageStreamTag{})

	return err
}

func (c *FakeImageStreamTags) Get(name string, options meta_v1.GetOptions) (result *v1.ImageStreamTag, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(imagestreamtagsResource, c.ns, name), &v1.ImageStreamTag{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ImageStreamTag), err
}
