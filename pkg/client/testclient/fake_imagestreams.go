package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"
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

var imageStreamsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "imagestreams"}
var imageStreamImportsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "imagestreamimports"}

func (c *FakeImageStreams) Get(name string) (*imageapi.ImageStream, error) {
	obj, err := c.Fake.Invokes(core.NewGetAction(imageStreamsResource, c.Namespace, name), &imageapi.ImageStream{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) List(opts kapi.ListOptions) (*imageapi.ImageStreamList, error) {
	obj, err := c.Fake.Invokes(core.NewListAction(imageStreamsResource, c.Namespace, opts), &imageapi.ImageStreamList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStreamList), err
}

func (c *FakeImageStreams) Create(inObj *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(imageStreamsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) Update(inObj *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	obj, err := c.Fake.Invokes(core.NewUpdateAction(imageStreamsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewDeleteAction(imageStreamsResource, c.Namespace, name), &imageapi.ImageStream{})
	return err
}

func (c *FakeImageStreams) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(core.NewWatchAction(imageStreamsResource, c.Namespace, opts))
}

func (c *FakeImageStreams) UpdateStatus(inObj *imageapi.ImageStream) (result *imageapi.ImageStream, err error) {
	action := core.CreateActionImpl{}
	action.Verb = "update"
	action.Resource = imageStreamsResource
	action.Subresource = "status"
	action.Object = inObj

	obj, err := c.Fake.Invokes(action, inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) Import(inObj *imageapi.ImageStreamImport) (*imageapi.ImageStreamImport, error) {
	action := core.CreateActionImpl{}
	action.Verb = "create"
	action.Resource = imageStreamImportsResource
	action.Object = inObj
	obj, err := c.Fake.Invokes(action, inObj)
	if obj == nil {
		return nil, err
	}
	return obj.(*imageapi.ImageStreamImport), nil
}
