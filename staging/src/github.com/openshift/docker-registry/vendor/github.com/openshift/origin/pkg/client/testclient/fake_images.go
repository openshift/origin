package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// FakeImages implements ImageInterface. Meant to be embedded into a struct to
// get a default implementation. This makes faking out just the methods you
// want to test easier.
type FakeImages struct {
	Fake *Fake
}

var _ client.ImageInterface = &FakeImages{}

var imagesResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "images"}
var imagesKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "Image"}

func (c *FakeImages) Get(name string, options metav1.GetOptions) (*imageapi.Image, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(imagesResource, name), &imageapi.Image{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.Image), err
}

func (c *FakeImages) List(opts metav1.ListOptions) (*imageapi.ImageList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(imagesResource, imagesKind, opts), &imageapi.ImageList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageList), err
}

func (c *FakeImages) Create(inObj *imageapi.Image) (*imageapi.Image, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(imagesResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.Image), err
}

func (c *FakeImages) Update(inObj *imageapi.Image) (*imageapi.Image, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(imagesResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.Image), err
}

func (c *FakeImages) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(imagesResource, name), &imageapi.Image{})
	return err
}
