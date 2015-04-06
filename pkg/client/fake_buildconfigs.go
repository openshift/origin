package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// FakeBuildConfigs implements BuildConfigInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeBuildConfigs struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeBuildConfigs) List(label labels.Selector, field fields.Selector) (*buildapi.BuildConfigList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-buildconfig"}, &buildapi.BuildConfigList{})
	return obj.(*buildapi.BuildConfigList), err
}

func (c *FakeBuildConfigs) Get(name string) (*buildapi.BuildConfig, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-buildconfig", Value: name}, &buildapi.BuildConfig{})
	return obj.(*buildapi.BuildConfig), err
}

func (c *FakeBuildConfigs) Create(config *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-buildconfig"}, &buildapi.BuildConfig{})
	return obj.(*buildapi.BuildConfig), err
}

func (c *FakeBuildConfigs) Update(config *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "update-buildconfig"}, &buildapi.BuildConfig{})
	return obj.(*buildapi.BuildConfig), err
}

func (c *FakeBuildConfigs) Delete(name string) error {
	_, err := c.Fake.Invokes(FakeAction{Action: "delete-buildconfig", Value: name}, &buildapi.BuildConfig{})
	return err
}

func (c *FakeBuildConfigs) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-buildconfigs"})
	return nil, nil
}

func (c *FakeBuildConfigs) Instantiate(request *buildapi.BuildRequest) (result *buildapi.Build, err error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "instantiate-buildconfig", Value: request}, &buildapi.Build{})
	return obj.(*buildapi.Build), err
}
