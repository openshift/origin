package fake

import (
	internalversion "github.com/openshift/origin/pkg/apps/generated/internalclientset/typed/apps/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeApps struct {
	*testing.Fake
}

func (c *FakeApps) DeploymentConfigs(namespace string) internalversion.DeploymentConfigInterface {
	return &FakeDeploymentConfigs{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeApps) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
