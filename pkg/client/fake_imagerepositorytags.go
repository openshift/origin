package client

import (
	"fmt"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImageRepositories implements ImageInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeImageRepositoryTags struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeImageRepositoryTags) Get(name, tag string) (result *imageapi.Image, err error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-imagerepository-tag", Value: fmt.Sprintf("%s:%s", name, tag)})
	return &imageapi.Image{}, nil
}
