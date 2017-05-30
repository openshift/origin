package fake

import (
	internalversion "github.com/openshift/origin/pkg/sdn/generated/internalclientset/typed/sdn/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeSdn struct {
	*testing.Fake
}

func (c *FakeSdn) ClusterNetworks(namespace string) internalversion.ClusterNetworkInterface {
	return &FakeClusterNetworks{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeSdn) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
