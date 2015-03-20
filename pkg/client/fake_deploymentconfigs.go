package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// FakeDeploymentConfigs implements BuildInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeDeploymentConfigs struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeDeploymentConfigs) List(label labels.Selector, field fields.Selector) (*deployapi.DeploymentConfigList, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "list-deploymentconfig"})
	return &deployapi.DeploymentConfigList{}, nil
}

func (c *FakeDeploymentConfigs) Get(name string) (*deployapi.DeploymentConfig, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-deploymentconfig"})
	return &deployapi.DeploymentConfig{}, nil
}

func (c *FakeDeploymentConfigs) Create(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-deploymentconfig"})
	return &deployapi.DeploymentConfig{}, nil
}

func (c *FakeDeploymentConfigs) Update(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "update-deploymentconfig"})
	return &deployapi.DeploymentConfig{}, nil
}

func (c *FakeDeploymentConfigs) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-deploymentconfig"})
	return nil
}

func (c *FakeDeploymentConfigs) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-deploymentconfig"})
	return nil, nil
}

func (c *FakeDeploymentConfigs) Generate(name string) (*deployapi.DeploymentConfig, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "generate-deploymentconfig"})
	return nil, nil
}

func (c *FakeDeploymentConfigs) Rollback(config *deployapi.DeploymentConfigRollback) (result *deployapi.DeploymentConfig, err error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "rollback"})
	return nil, nil
}
