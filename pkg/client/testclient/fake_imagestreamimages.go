package testclient

import (
	"fmt"

	ktestclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"

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

func (c *FakeImageStreamImages) Get(name, id string) (*imageapi.ImageStreamImage, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "get-imagestream-image", Value: fmt.Sprintf("%s@%s", name, id)}, &imageapi.ImageStreamImage{})
	return obj.(*imageapi.ImageStreamImage), err
}
