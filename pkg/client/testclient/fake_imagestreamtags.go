package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImageStreamTags implements ImageStreamTagInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageStreamTags struct {
	Fake      *Fake
	Namespace string
}

var _ client.ImageStreamTagInterface = &FakeImageStreamTags{}

func (c *FakeImageStreamTags) Get(name, tag string) (*imageapi.ImageStreamTag, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewGetAction("imagestreamtags", c.Namespace, imageapi.JoinImageStreamTag(name, tag)), &imageapi.ImageStreamTag{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStreamTag), err
}

func (c *FakeImageStreamTags) Delete(name, tag string) error {
	_, err := c.Fake.Invokes(ktestclient.NewDeleteAction("imagestreamtags", c.Namespace, imageapi.JoinImageStreamTag(name, tag)), &imageapi.ImageStreamTag{})
	return err
}
