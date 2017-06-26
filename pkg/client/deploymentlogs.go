package client

import (
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
)

// DeploymentLogsNamespacer has methods to work with DeploymentLogs resources in a namespace
type DeploymentLogsNamespacer interface {
	DeploymentLogs(namespace string) DeploymentLogInterface
}

// DeploymentLogInterface exposes methods on DeploymentLogs resources.
type DeploymentLogInterface interface {
	Get(name string, opts deployapi.DeploymentLogOptions) *restclient.Request
}

// deploymentLogs implements DeploymentLogsNamespacer interface
type deploymentLogs struct {
	r  *Client
	ns string
}

// newDeploymentLogs returns a deploymentLogs
func newDeploymentLogs(c *Client, namespace string) *deploymentLogs {
	return &deploymentLogs{
		r:  c,
		ns: namespace,
	}
}

// Get gets the deploymentlogs and return a deploymentLog request
func (c *deploymentLogs) Get(name string, opts deployapi.DeploymentLogOptions) *restclient.Request {
	return c.r.Get().Namespace(c.ns).Resource("deploymentConfigs").Name(name).SubResource("log").VersionedParams(&opts, kapi.ParameterCodec)
}
