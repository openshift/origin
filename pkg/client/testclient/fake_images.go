package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImages implements ImageInterface. Meant to be embedded into a struct to
// get a default implementation. This makes faking out just the methods you
// want to test easier.
type FakeImages struct {
	Fake *Fake
}

var _ client.ImageInterface = &FakeImages{}

func (c *FakeImages) Get(name string) (*imageapi.Image, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("images", name), &imageapi.Image{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.Image), err
}

func (c *FakeImages) List(opts kapi.ListOptions) (*imageapi.ImageList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("images", opts), &imageapi.ImageList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageList), err
}

func (c *FakeImages) Create(inObj *imageapi.Image) (*imageapi.Image, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("images", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.Image), err
}

func (c *FakeImages) Update(inObj *imageapi.Image) (*imageapi.Image, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootUpdateAction("images", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.Image), err
}

func (c *FakeImages) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("images", name), &imageapi.Image{})
	return err
}
