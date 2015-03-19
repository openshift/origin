package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// FakeBuilds implements BuildInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeBuilds struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeBuilds) List(label labels.Selector, field fields.Selector) (*buildapi.BuildList, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "list-builds"})
	return &buildapi.BuildList{}, nil
}

func (c *FakeBuilds) Get(name string) (*buildapi.Build, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-build"})
	return &buildapi.Build{}, nil
}

func (c *FakeBuilds) Create(build *buildapi.Build) (*buildapi.Build, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-build", Value: build})
	return &buildapi.Build{}, nil
}

func (c *FakeBuilds) Update(build *buildapi.Build) (*buildapi.Build, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "update-build"})
	return &buildapi.Build{}, nil
}

func (c *FakeBuilds) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-build", Value: name})
	return nil
}

func (c *FakeBuilds) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-builds"})
	return nil, nil
}
