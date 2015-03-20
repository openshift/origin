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
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "list-buildconfig"})
	return &buildapi.BuildConfigList{}, nil
}

func (c *FakeBuildConfigs) Get(name string) (*buildapi.BuildConfig, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-buildconfig", Value: name})
	return &buildapi.BuildConfig{}, nil
}

func (c *FakeBuildConfigs) Create(config *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-buildconfig"})
	return &buildapi.BuildConfig{}, nil
}

func (c *FakeBuildConfigs) Update(config *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "update-buildconfig"})
	return &buildapi.BuildConfig{}, nil
}

func (c *FakeBuildConfigs) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-buildconfig", Value: name})
	return nil
}

func (c *FakeBuildConfigs) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-buildconfigs"})
	return nil, nil
}
