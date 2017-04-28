package fake

import (
	internalversion "github.com/openshift/origin/pkg/deploy/generated/internalclientset/typed/deploy/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeDeploy struct {
	*testing.Fake
}

func (c *FakeDeploy) DeploymentConfigs(namespace string) internalversion.DeploymentConfigInterface {
	return &FakeDeploymentConfigs{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeDeploy) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
