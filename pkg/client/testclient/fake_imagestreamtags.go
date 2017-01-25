package testclient

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImageStreamTags implements ImageStreamTagInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageStreamTags struct {
	Fake      *Fake
	Namespace string
}

var _ client.ImageStreamTagInterface = &FakeImageStreamTags{}

var imageStreamTagsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "imagestreamtags"}

func (c *FakeImageStreamTags) Get(name, tag string) (*imageapi.ImageStreamTag, error) {
	obj, err := c.Fake.Invokes(core.NewGetAction(imageStreamTagsResource, c.Namespace, imageapi.JoinImageStreamTag(name, tag)), &imageapi.ImageStreamTag{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStreamTag), err
}

func (c *FakeImageStreamTags) Update(inObj *imageapi.ImageStreamTag) (*imageapi.ImageStreamTag, error) {
	obj, err := c.Fake.Invokes(core.NewUpdateAction(imageStreamTagsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStreamTag), err
}

func (c *FakeImageStreamTags) Create(inObj *imageapi.ImageStreamTag) (*imageapi.ImageStreamTag, error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(imageStreamTagsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStreamTag), err
}

func (c *FakeImageStreamTags) Delete(name, tag string) error {
	_, err := c.Fake.Invokes(core.NewDeleteAction(imageStreamTagsResource, c.Namespace, imageapi.JoinImageStreamTag(name, tag)), &imageapi.ImageStreamTag{})
	return err
}
