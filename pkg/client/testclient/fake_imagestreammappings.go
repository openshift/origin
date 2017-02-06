package testclient

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

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

var imageStreamMappingsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "imagestreammappings"}

func (c *FakeImageStreamMappings) Create(inObj *imageapi.ImageStreamMapping) error {
	_, err := c.Fake.Invokes(core.NewCreateAction(imageStreamMappingsResource, c.Namespace, inObj), inObj)
	return err
}
