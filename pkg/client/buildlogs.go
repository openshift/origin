package client

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	api "github.com/openshift/origin/pkg/build/api"
)

// BuildLogsNamespacer has methods to work with BuildLogs resources in a namespace
type BuildLogsNamespacer interface {
	BuildLogs(namespace string) BuildLogsInterface
}

// BuildLogsInterface exposes methods on BuildLogs resources.
type BuildLogsInterface interface {
	Get(name string, opts api.BuildLogOptions) *kclient.Request
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

// Get builds and returns a buildLog request
func (c *buildLogs) Get(name string, opts api.BuildLogOptions) *kclient.Request {
	return c.r.Get().Namespace(c.ns).Resource("builds").Name(name).SubResource("log").VersionedParams(&opts, kapi.Scheme)
}
