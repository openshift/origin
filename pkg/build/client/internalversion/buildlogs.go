package internalversion

import (
	rest "k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

type BuildLogInterface interface {
	Logs(name string, options buildapi.BuildLogOptions) *rest.Request
}

func NewBuildLogClient(c rest.Interface, ns string) BuildLogInterface {
	return &buildLogs{client: c, ns: ns}
}

type buildLogs struct {
	client rest.Interface
	ns     string
}

func (c *buildLogs) Logs(name string, options buildapi.BuildLogOptions) *rest.Request {
	return c.client.
		Get().
		Namespace(c.ns).
		Resource("builds").
		Name(name).
		SubResource("log").
		VersionedParams(&options, legacyscheme.ParameterCodec)
}
