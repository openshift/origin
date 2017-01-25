package testclient

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImageStreamImages implements ImageStreamImageInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageStreamImages struct {
	Fake      *Fake
	Namespace string
}

var _ client.ImageStreamImageInterface = &FakeImageStreamImages{}

var imageStreamImagesResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "imagestreamimages"}

func (c *FakeImageStreamImages) Get(repo, imageID string) (*imageapi.ImageStreamImage, error) {
	name := fmt.Sprintf("%s@%s", repo, imageID)

	obj, err := c.Fake.Invokes(core.NewGetAction(imageStreamImagesResource, c.Namespace, name), &imageapi.ImageStreamImage{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStreamImage), err
}
