package client

import (
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImageStreams implements ImageStreamMappingInterface. Meant to
// be embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageStreamMappings struct {
	Fake      *Fake
	Namespace string
}

var _ ImageStreamMappingInterface = &FakeImageStreamMappings{}

func (c *FakeImageStreamMappings) Create(mapping *imageapi.ImageStreamMapping) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-imagestream-mapping"})
	return nil
}
