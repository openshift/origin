package testclient

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// FakeImageStreamTags implements ImageStreamTagInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageStreamTags struct {
	Fake      *Fake
	Namespace string
}

var _ client.ImageStreamTagInterface = &FakeImageStreamTags{}

var imageStreamTagsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "imagestreamtags"}

func (c *FakeImageStreamTags) Get(name, tag string) (*imageapi.ImageStreamTag, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(imageStreamTagsResource, c.Namespace, imageapi.JoinImageStreamTag(name, tag)), &imageapi.ImageStreamTag{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStreamTag), err
}

func (c *FakeImageStreamTags) Update(inObj *imageapi.ImageStreamTag) (*imageapi.ImageStreamTag, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewUpdateAction(imageStreamTagsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStreamTag), err
}

func (c *FakeImageStreamTags) Create(inObj *imageapi.ImageStreamTag) (*imageapi.ImageStreamTag, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(imageStreamTagsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStreamTag), err
}

func (c *FakeImageStreamTags) Delete(name, tag string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewDeleteAction(imageStreamTagsResource, c.Namespace, imageapi.JoinImageStreamTag(name, tag)), &imageapi.ImageStreamTag{})
	return err
}
