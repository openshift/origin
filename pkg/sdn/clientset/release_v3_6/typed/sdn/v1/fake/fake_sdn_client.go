package fake

import (
	v1 "github.com/openshift/origin/pkg/sdn/clientset/release_v3_6/typed/sdn/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeSdnV1 struct {
	*core.Fake
}

func (c *FakeSdnV1) ClusterNetworks(namespace string) v1.ClusterNetworkInterface {
	return &FakeClusterNetworks{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeSdnV1) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
