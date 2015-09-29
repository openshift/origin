package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

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

func (c *FakeImageStreamMappings) Create(inObj *imageapi.ImageStreamMapping) error {
	_, err := c.Fake.Invokes(ktestclient.NewCreateAction("imagestreammappings", c.Namespace, inObj), inObj)
	return err
}
