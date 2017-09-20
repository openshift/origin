package testclient

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// FakeImageSignatures implements ImageSignatureInterface. Meant to
// be embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageSignatures struct {
	Fake *Fake
}

var _ client.ImageSignatureInterface = &FakeImageSignatures{}

var imageSignaturesResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "imagesignatures"}

func (c *FakeImageSignatures) Create(inObj *imageapi.ImageSignature) (*imageapi.ImageSignature, error) {
	_, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(imageSignaturesResource, inObj), inObj)
	return inObj, err
}

func (c *FakeImageSignatures) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(imageSignaturesResource, name), &imageapi.ImageSignature{})
	return err
}
