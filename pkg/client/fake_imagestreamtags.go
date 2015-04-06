package client

import (
	"fmt"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImageStreamTags implements ImageStreamTagInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageStreamTags struct {
	Fake      *Fake
	Namespace string
}

var _ ImageStreamTagInterface = &FakeImageStreamTags{}

func (c *FakeImageStreamTags) Get(name, tag string) (result *imageapi.Image, err error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-imagestream-tag", Value: fmt.Sprintf("%s:%s", name, tag)})
	return &imageapi.Image{}, nil
}

func (c *FakeImageStreamTags) Delete(name, tag string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-imagestream-tag", Value: fmt.Sprintf("%s:%s", name, tag)})
	return nil
}
