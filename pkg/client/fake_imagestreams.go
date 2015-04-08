package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImageStreams implements ImageStreamInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageStreams struct {
	Fake      *Fake
	Namespace string
}

var _ ImageStreamInterface = &FakeImageStreams{}

func (c *FakeImageStreams) List(label labels.Selector, field fields.Selector) (*imageapi.ImageStreamList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-imagestreams"}, &imageapi.ImageStreamList{})
	return obj.(*imageapi.ImageStreamList), err
}

func (c *FakeImageStreams) Get(name string) (*imageapi.ImageStream, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-imagestream", Value: name}, &imageapi.ImageStream{})
	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) Create(repo *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-imagestream"}, &imageapi.ImageStream{})
	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) Update(repo *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "update-imagestream"}, &imageapi.ImageStream{})
	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-imagestream", Value: name})
	return nil
}

func (c *FakeImageStreams) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-imagestreams"})
	return nil, nil
}
