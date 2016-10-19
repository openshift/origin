package fake

import (
	unversioned "github.com/openshift/origin/pkg/deploy/client/clientset_generated/internalclientset/typed/core/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/kubectl/resource"
)

type FakeCore struct {
	*core.Fake
}

func (c *FakeCore) DeploymentConfigs(namespace string) unversioned.DeploymentConfigInterface {
	return &FakeDeploymentConfigs{c, namespace}
}

// GetRESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeCore) GetRESTClient() resource.RESTClient {
	return nil
}
