package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// FakeImageStreams implements ImageStreamInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageStreams struct {
	Fake      *Fake
	Namespace string
}

var _ client.ImageStreamInterface = &FakeImageStreams{}

var imageStreamsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "imagestreams"}
var imageStreamImportsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "imagestreamimports"}
var imageStreamsKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "ImageStream"}

func (c *FakeImageStreams) Get(name string, options metav1.GetOptions) (*imageapi.ImageStream, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(imageStreamsResource, c.Namespace, name), &imageapi.ImageStream{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) List(opts metav1.ListOptions) (*imageapi.ImageStreamList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewListAction(imageStreamsResource, imageStreamsKind, c.Namespace, opts), &imageapi.ImageStreamList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStreamList), err
}

func (c *FakeImageStreams) Create(inObj *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(imageStreamsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) Update(inObj *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewUpdateAction(imageStreamsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStream), err
}

func (c *FakeImageStreams) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewDeleteAction(imageStreamsResource, c.Namespace, name), &imageapi.ImageStream{})
	return err
}

func (c *FakeImageStreams) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewWatchAction(imageStreamsResource, c.Namespace, opts))
}

func (c *FakeImageStreams) UpdateStatus(inObj *imageapi.ImageStream) (result *imageapi.ImageStream, err error) {
	action := clientgotesting.CreateActionImpl{}
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
	action := clientgotesting.CreateActionImpl{}
	action.Verb = "create"
	action.Resource = imageStreamImportsResource
	action.Object = inObj
	obj, err := c.Fake.Invokes(action, inObj)
	if obj == nil {
		return nil, err
	}
	return obj.(*imageapi.ImageStreamImport), nil
}
