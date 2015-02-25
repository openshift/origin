package client

import (
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
)

// BuildLogsNamespacer has methods to work with BuildLogs resources in a namespace
type BuildLogsNamespacer interface {
	BuildLogs(namespace string) BuildLogInterface
}

// BuildLogsInterface exposes methods on BuildLogs resources.
type BuildLogInterface interface {
	Redirect(name string) *kclient.Request
}

// buildLogs implements BuildLogsNamespacer interface
type buildLogs struct {
	r  *Client
	ns string
}

// newBuildLogs returns a buildLogs
func newBuildLogs(c *Client, namespace string) *buildLogs {
	return &buildLogs{
		r:  c,
		ns: namespace,
	}
}

// Redirect builds and returns a buildLog request
func (c *buildLogs) Redirect(name string) *kclient.Request {
	return c.r.Get().Namespace(c.ns).Prefix("redirect").Resource("buildLogs").Name(name)
}
