package fake

import (
	internalversion "github.com/openshift/origin/pkg/network/generated/internalclientset/typed/network/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeNetwork struct {
	*testing.Fake
}

func (c *FakeNetwork) ClusterNetworks() internalversion.ClusterNetworkInterface {
	return &FakeClusterNetworks{c}
}

func (c *FakeNetwork) EgressNetworkPolicies(namespace string) internalversion.EgressNetworkPolicyInterface {
	return &FakeEgressNetworkPolicies{c, namespace}
}

func (c *FakeNetwork) HostSubnets() internalversion.HostSubnetInterface {
	return &FakeHostSubnets{c}
}

func (c *FakeNetwork) NetNamespaces() internalversion.NetNamespaceInterface {
	return &FakeNetNamespaces{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeNetwork) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
