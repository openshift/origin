package client

import (
	"fmt"

	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/openshift/origin/pkg/deploy/api"
)

// DeploymentLogsNamespacer has methods to work with DeploymentLogs resources in a namespace
type DeploymentLogsNamespacer interface {
	DeploymentLogs(namespace string) DeploymentLogInterface
}

// DeploymentLogInterface exposes methods on DeploymentLogs resources.
type DeploymentLogInterface interface {
	Get(name string, opts api.DeploymentLogOptions) *kclient.Request
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
func (c *deploymentLogs) Get(name string, opts api.DeploymentLogOptions) *kclient.Request {
	req := c.r.Get().Namespace(c.ns).Resource("deploymentConfigs").Name(name).SubResource("log")
	if opts.NoWait {
		req.Param("nowait", "true")
	}
	if opts.Follow {
		req.Param("follow", "true")
	}
	if opts.Version != nil {
		req.Param("version", fmt.Sprintf("%d", *opts.Version))
	}
	return req
}
