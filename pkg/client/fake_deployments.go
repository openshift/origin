package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// FakeDeployments implements BuildInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeDeployments struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeDeployments) List(label labels.Selector, field fields.Selector) (*deployapi.DeploymentList, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "list-deployment"})
	return &deployapi.DeploymentList{}, nil
}

func (c *FakeDeployments) Get(name string) (*deployapi.Deployment, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-deployment"})
	return &deployapi.Deployment{}, nil
}

func (c *FakeDeployments) Create(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-deployment"})
	return &deployapi.Deployment{}, nil
}

func (c *FakeDeployments) Update(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "update-deployment", Value: deployment})
	return &deployapi.Deployment{}, nil
}

func (c *FakeDeployments) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-deployment"})
	return nil
}

func (c *FakeDeployments) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-deployments"})
	return nil, nil
}
