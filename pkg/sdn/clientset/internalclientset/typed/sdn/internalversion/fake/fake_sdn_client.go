package fake

import (
	internalversion "github.com/openshift/origin/pkg/sdn/clientset/internalclientset/typed/sdn/internalversion"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeSdn struct {
	*core.Fake
}

func (c *FakeSdn) ClusterNetworks(namespace string) internalversion.ClusterNetworkInterface {
	return &FakeClusterNetworks{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeSdn) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
