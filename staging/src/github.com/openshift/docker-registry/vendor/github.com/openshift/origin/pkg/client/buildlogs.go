package client

import (
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	api "github.com/openshift/origin/pkg/build/apis/build"
)

// BuildLogsNamespacer has methods to work with BuildLogs resources in a namespace
type BuildLogsNamespacer interface {
	BuildLogs(namespace string) BuildLogsInterface
}

// BuildLogsInterface exposes methods on BuildLogs resources.
type BuildLogsInterface interface {
	Get(name string, opts api.BuildLogOptions) *restclient.Request
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
func (c *buildLogs) Get(name string, opts api.BuildLogOptions) *restclient.Request {
	return c.r.Get().Namespace(c.ns).Resource("builds").Name(name).SubResource("log").VersionedParams(&opts, kapi.ParameterCodec)
}
