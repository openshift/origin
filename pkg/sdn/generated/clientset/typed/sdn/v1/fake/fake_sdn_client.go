package fake

import (
	v1 "github.com/openshift/origin/pkg/sdn/generated/clientset/typed/sdn/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeSdnV1 struct {
	*testing.Fake
}

func (c *FakeSdnV1) ClusterNetworks(namespace string) v1.ClusterNetworkInterface {
	return &FakeClusterNetworks{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeSdnV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
