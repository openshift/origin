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
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-deployment"}, &deployapi.DeploymentList{})
	return obj.(*deployapi.DeploymentList), err
}

func (c *FakeDeployments) Get(name string) (*deployapi.Deployment, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-deployment"}, &deployapi.Deployment{})
	return obj.(*deployapi.Deployment), err
}

func (c *FakeDeployments) Create(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-deployment"}, &deployapi.Deployment{})
	return obj.(*deployapi.Deployment), err
}

func (c *FakeDeployments) Update(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "update-deployment", Value: deployment}, &deployapi.Deployment{})
	return obj.(*deployapi.Deployment), err
}

func (c *FakeDeployments) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-deployment"})
	return nil
}

func (c *FakeDeployments) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-deployments"})
	return nil, nil
}
