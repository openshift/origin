package client

import (
	"fmt"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImageStreamImages implements ImageStreamImageInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageStreamImages struct {
	Fake      *Fake
	Namespace string
}

var _ ImageStreamImageInterface = &FakeImageStreamImages{}

func (c *FakeImageStreamImages) Get(name, id string) (result *imageapi.Image, err error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-imagestream-image", Value: fmt.Sprintf("%s@%s", name, id)})
	return &imageapi.Image{}, nil
}
