package fake

import (
	v1 "github.com/openshift/client-go/network/clientset/versioned/typed/network/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeNetworkV1 struct {
	*testing.Fake
}

func (c *FakeNetworkV1) ClusterNetworks() v1.ClusterNetworkInterface {
	return &FakeClusterNetworks{c}
}

func (c *FakeNetworkV1) EgressNetworkPolicies(namespace string) v1.EgressNetworkPolicyInterface {
	return &FakeEgressNetworkPolicies{c, namespace}
}

func (c *FakeNetworkV1) HostSubnets() v1.HostSubnetInterface {
	return &FakeHostSubnets{c}
}

func (c *FakeNetworkV1) NetNamespaces() v1.NetNamespaceInterface {
	return &FakeNetNamespaces{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeNetworkV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
