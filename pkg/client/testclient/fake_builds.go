package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/testclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// FakeBuilds implements BuildInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeBuilds struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeBuilds) Get(name string) (*buildapi.Build, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewGetAction("builds", c.Namespace, name), &buildapi.Build{})
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.Build), err
}

func (c *FakeBuilds) List(label labels.Selector, field fields.Selector) (*buildapi.BuildList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewListAction("builds", c.Namespace, label, field), &buildapi.BuildList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.BuildList), err
}

func (c *FakeBuilds) Create(inObj *buildapi.Build) (*buildapi.Build, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewCreateAction("builds", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.Build), err
}

func (c *FakeBuilds) Update(inObj *buildapi.Build) (*buildapi.Build, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewUpdateAction("builds", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.Build), err
}

func (c *FakeBuilds) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewDeleteAction("builds", c.Namespace, name), &buildapi.Build{})
	return err
}

func (c *FakeBuilds) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Invokes(ktestclient.NewWatchAction("builds", c.Namespace, label, field, resourceVersion), nil)
	return c.Fake.Watch, nil
}

func (c *FakeBuilds) Clone(request *buildapi.BuildRequest) (result *buildapi.Build, err error) {
	action := ktestclient.NewCreateAction("buildconfigs", c.Namespace, request)
	action.Subresource = "clone"
	obj, err := c.Fake.Invokes(action, &buildapi.Build{})
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.Build), err
}
