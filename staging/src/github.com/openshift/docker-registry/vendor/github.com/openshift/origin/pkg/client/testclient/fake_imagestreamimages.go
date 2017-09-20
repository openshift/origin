package testclient

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// FakeImageStreamImages implements ImageStreamImageInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageStreamImages struct {
	Fake      *Fake
	Namespace string
}

var _ client.ImageStreamImageInterface = &FakeImageStreamImages{}

var imageStreamImagesResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "imagestreamimages"}

func (c *FakeImageStreamImages) Get(repo, imageID string) (*imageapi.ImageStreamImage, error) {
	name := fmt.Sprintf("%s@%s", repo, imageID)

	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(imageStreamImagesResource, c.Namespace, name), &imageapi.ImageStreamImage{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStreamImage), err
}
