package internalversion

import (
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildinternalversion "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
)

type BuildLogInterface interface {
	Logs(name string, options buildapi.BuildLogOptions) *restclient.Request
}

func NewBuildLogClient(c buildinternalversion.BuildInterface, ns string) BuildLogInterface {
	return &buildLogs{client: c, ns: ns}
}

type buildLogs struct {
	client buildinternalversion.BuildInterface
	ns     string
}

func (c *buildLogs) Logs(name string, options buildapi.BuildLogOptions) *restclient.Request {
	return c.client.RESTClient().
		Get().
		Namespace(c.ns).
		Resource("builds").
		Name(name).
		SubResource("log").
		VersionedParams(&options, kapi.ParameterCodec)
}
