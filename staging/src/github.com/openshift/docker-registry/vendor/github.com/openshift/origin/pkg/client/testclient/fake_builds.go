package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

// FakeBuilds implements BuildInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeBuilds struct {
	Fake      *Fake
	Namespace string
}

var buildsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "builds"}
var buildKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "Build"}

func (c *FakeBuilds) Get(name string, options metav1.GetOptions) (*buildapi.Build, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(buildsResource, c.Namespace, name), &buildapi.Build{})
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.Build), err
}

func (c *FakeBuilds) List(opts metav1.ListOptions) (*buildapi.BuildList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewListAction(buildsResource, buildKind, c.Namespace, opts), &buildapi.BuildList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.BuildList), err
}

func (c *FakeBuilds) Create(inObj *buildapi.Build) (*buildapi.Build, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(buildsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.Build), err
}

func (c *FakeBuilds) Update(inObj *buildapi.Build) (*buildapi.Build, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewUpdateAction(buildsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.Build), err
}

func (c *FakeBuilds) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*buildapi.Build, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewPatchSubresourceAction(buildsResource, c.Namespace, name, data, subresources...), &buildapi.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*buildapi.Build), err
}

func (c *FakeBuilds) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewDeleteAction(buildsResource, c.Namespace, name), &buildapi.Build{})
	return err
}

func (c *FakeBuilds) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewWatchAction(buildsResource, c.Namespace, opts))
}

func (c *FakeBuilds) Clone(request *buildapi.BuildRequest) (result *buildapi.Build, err error) {
	action := clientgotesting.NewCreateAction(buildsResource, c.Namespace, request)
	action.Subresource = "clone"
	obj, err := c.Fake.Invokes(action, &buildapi.Build{})
	if obj == nil {
		return nil, err
	}

	if br, ok := obj.(*buildapi.BuildRequest); ok {
		return &buildapi.Build{
			ObjectMeta: br.ObjectMeta,
		}, err
	}
	return obj.(*buildapi.Build), err
}

func (c *FakeBuilds) UpdateDetails(inObj *buildapi.Build) (*buildapi.Build, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewUpdateAction(buildapi.LegacySchemeGroupVersion.WithResource("builds/details"), c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.Build), err
}
