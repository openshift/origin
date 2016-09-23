package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
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

func (c *FakeBuilds) List(opts kapi.ListOptions) (*buildapi.BuildList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewListAction("builds", c.Namespace, opts), &buildapi.BuildList{})
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

func (c *FakeBuilds) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewWatchAction("builds", c.Namespace, opts))
}

func (c *FakeBuilds) Clone(request *buildapi.BuildRequest) (result *buildapi.Build, err error) {
	action := ktestclient.NewCreateAction("builds", c.Namespace, request)
	action.Subresource = "clone"
	obj, err := c.Fake.Invokes(action, &buildapi.Build{})
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.Build), err
}

func (c *FakeBuilds) UpdateDetails(inObj *buildapi.Build) (*buildapi.Build, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewUpdateAction("builds/details", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.Build), err
}
