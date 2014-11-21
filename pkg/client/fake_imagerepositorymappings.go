package client

import (
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImageRepositories implements ImageInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeImageRepositoryMappings struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeImageRepositoryMappings) Create(mapping *imageapi.ImageRepositoryMapping) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-imagerepository-mapping"})
	return nil
}
