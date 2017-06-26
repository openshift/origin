package testclient

import (
	restclient "k8s.io/client-go/rest"
	clientgotesting "k8s.io/client-go/testing"

	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
)

// FakeDeploymentLogs implements DeploymentLogsInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeDeploymentLogs struct {
	Fake      *Fake
	Namespace string
}

// Get builds and returns a buildLog request
func (c *FakeDeploymentLogs) Get(name string, opt deployapi.DeploymentLogOptions) *restclient.Request {
	action := clientgotesting.GenericActionImpl{}
	action.Verb = "get"
	action.Namespace = c.Namespace
	action.Resource = deploymentConfigsResource
	action.Subresource = "log"
	action.Value = opt

	_, _ = c.Fake.Invokes(action, &deployapi.DeploymentConfig{})
	return &restclient.Request{}
}
