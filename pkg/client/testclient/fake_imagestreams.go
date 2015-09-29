package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

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

func (c *FakeImageStreams) Get(name string) (*imageapi.ImageStream, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewGetAction("imagestreams", c.Namespace, name), &imageapi.ImageStream{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) List(label labels.Selector, field fields.Selector) (*imageapi.ImageStreamList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewListAction("imagestreams", c.Namespace, label, field), &imageapi.ImageStreamList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStreamList), err
}

func (c *FakeImageStreams) Create(inObj *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewCreateAction("imagestreams", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) Update(inObj *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewUpdateAction("imagestreams", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewDeleteAction("imagestreams", c.Namespace, name), &imageapi.ImageStream{})
	return err
}

func (c *FakeImageStreams) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewWatchAction("imagestreams", c.Namespace, label, field, resourceVersion))
}

func (c *FakeImageStreams) UpdateStatus(inObj *imageapi.ImageStream) (result *imageapi.ImageStream, err error) {
	action := ktestclient.CreateActionImpl{}
	action.Verb = "update"
	action.Resource = "imagestreams"
	action.Subresource = "status"
	action.Object = inObj

	obj, err := c.Fake.Invokes(action, inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStream), err
}
