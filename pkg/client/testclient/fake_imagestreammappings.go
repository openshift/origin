package testclient

import (
	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImageStreamMappings implements ImageStreamMappingInterface. Meant to
// be embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageStreamMappings struct {
	Fake      *Fake
	Namespace string
}

var _ client.ImageStreamMappingInterface = &FakeImageStreamMappings{}

func (c *FakeImageStreamMappings) Create(mapping *imageapi.ImageStreamMapping) error {
	c.Fake.Lock.Lock()
	defer c.Fake.Lock.Unlock()

	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-imagestream-mapping"})
	return nil
}
