package fake

import (
	v1 "github.com/openshift/origin/pkg/deploy/client/clientset_generated/release_v1_3/typed/core/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeCore struct {
	*core.Fake
}

func (c *FakeCore) DeploymentConfigs(namespace string) v1.DeploymentConfigInterface {
	return &FakeDeploymentConfigs{c, namespace}
}

// GetRESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeCore) GetRESTClient() *restclient.RESTClient {
	return nil
}
