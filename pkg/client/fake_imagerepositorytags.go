package client

import (
	"fmt"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImageRepositoryTags implements ImageRepositoryTagInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageRepositoryTags struct {
	Fake      *Fake
	Namespace string
}

var _ ImageRepositoryTagInterface = &FakeImageRepositoryTags{}

func (c *FakeImageRepositoryTags) Get(name, tag string) (result *imageapi.Image, err error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-imagerepository-tag", Value: fmt.Sprintf("%s:%s", name, tag)})
	return &imageapi.Image{}, nil
}

func (c *FakeImageRepositoryTags) Delete(name, tag string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-imagerepository-tag", Value: fmt.Sprintf("%s:%s", name, tag)})
	return nil
}
