package internalversion

import (
	rest "k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

type RolloutLogInterface interface {
	Logs(name string, options appsapi.DeploymentLogOptions) *rest.Request
}

func NewRolloutLogClient(c rest.Interface, ns string) RolloutLogInterface {
	return &rolloutLogs{client: c, ns: ns}
}

type rolloutLogs struct {
	client rest.Interface
	ns     string
}

func (c *rolloutLogs) Logs(name string, options appsapi.DeploymentLogOptions) *rest.Request {
	return c.client.
		Get().
		Namespace(c.ns).
		Resource("deploymentConfigs").
		Name(name).
		SubResource("log").
		VersionedParams(&options, legacyscheme.ParameterCodec)
}
