package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImages implements ImageInterface. Meant to be embedded into a struct to
// get a default implementation. This makes faking out just the methods you
// want to test easier.
type FakeImages struct {
	Fake *Fake
}

var _ ImageInterface = &FakeImages{}

func (c *FakeImages) List(label labels.Selector, field fields.Selector) (*imageapi.ImageList, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "list-images"})
	return &imageapi.ImageList{}, nil
}

func (c *FakeImages) Get(name string) (*imageapi.Image, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-image", Value: name})
	return &imageapi.Image{}, nil
}

func (c *FakeImages) Create(image *imageapi.Image) (*imageapi.Image, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-image"})
	return &imageapi.Image{}, nil
}

func (c *FakeImages) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-image", Value: name})
	return nil
}
