package testclient

import (
	ktestclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImageStreams implements ImageStreamInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageStreams struct {
	Fake      *Fake
	Namespace string
}

var _ client.ImageStreamInterface = &FakeImageStreams{}

func (c *FakeImageStreams) List(label labels.Selector, field fields.Selector) (*imageapi.ImageStreamList, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "list-imagestreams"}, &imageapi.ImageStreamList{})
	return obj.(*imageapi.ImageStreamList), err
}

func (c *FakeImageStreams) Get(name string) (*imageapi.ImageStream, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "get-imagestream", Value: name}, &imageapi.ImageStream{})
	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) Create(stream *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "create-imagestream"}, &imageapi.ImageStream{})
	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) Update(stream *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "update-imagestream", Value: stream}, stream)
	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, ktestclient.FakeAction{Action: "delete-imagestream", Value: name})
	return nil
}

func (c *FakeImageStreams) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, ktestclient.FakeAction{Action: "watch-imagestreams"})
	return nil, nil
}

func (c *FakeImageStreams) UpdateStatus(stream *imageapi.ImageStream) (result *imageapi.ImageStream, err error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "update-status-imagestream", Value: stream}, stream)
	return obj.(*imageapi.ImageStream), err
}
