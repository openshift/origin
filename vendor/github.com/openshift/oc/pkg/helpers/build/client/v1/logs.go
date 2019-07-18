package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	buildv1 "github.com/openshift/api/build/v1"
)

type BuildLogInterface interface {
	Logs(name string, options buildv1.BuildLogOptions) *rest.Request
}

func NewBuildLogClient(c rest.Interface, ns string, scheme *runtime.Scheme) BuildLogInterface {
	return &buildLogs{client: c, ns: ns, codec: runtime.NewParameterCodec(scheme)}
}

type buildLogs struct {
	client rest.Interface
	ns     string
	codec  runtime.ParameterCodec
}

func (c *buildLogs) Logs(name string, options buildv1.BuildLogOptions) *rest.Request {
	return c.client.
		Get().
		Namespace(c.ns).
		Resource("builds").
		Name(name).
		SubResource("log").
		VersionedParams(&options, c.codec)
}
