package fake

import (
	internalversion "github.com/openshift/origin/pkg/sdn/client/clientset_generated/internalclientset/typed/core/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeCore struct {
	*testing.Fake
}

func (c *FakeCore) ClusterNetworks(namespace string) internalversion.ClusterNetworkInterface {
	return &FakeClusterNetworks{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeCore) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
