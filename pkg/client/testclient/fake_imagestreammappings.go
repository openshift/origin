package testclient

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// FakeImageStreamMappings implements ImageStreamMappingInterface. Meant to
// be embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageStreamMappings struct {
	Fake      *Fake
	Namespace string
}

var _ client.ImageStreamMappingInterface = &FakeImageStreamMappings{}

var imageStreamMappingsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "imagestreammappings"}

func (c *FakeImageStreamMappings) Create(inObj *imageapi.ImageStreamMapping) error {
	_, err := c.Fake.Invokes(clientgotesting.NewCreateAction(imageStreamMappingsResource, c.Namespace, inObj), inObj)
	return err
}
