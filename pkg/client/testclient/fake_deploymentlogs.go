package testclient

import (
	"k8s.io/kubernetes/pkg/client/restclient"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	"github.com/openshift/origin/pkg/deploy/api"
)

// FakeDeploymentLogs implements DeploymentLogsInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeDeploymentLogs struct {
	Fake      *Fake
	Namespace string
}

// Get builds and returns a buildLog request
func (c *FakeDeploymentLogs) Get(name string, opt api.DeploymentLogOptions) *restclient.Request {
	action := ktestclient.GenericActionImpl{}
	action.Verb = "get"
	action.Namespace = c.Namespace
	action.Resource = "deploymentconfigs"
	action.Subresource = "log"
	action.Value = opt

	_, _ = c.Fake.Invokes(action, &api.DeploymentConfig{})
	return &restclient.Request{}
}
