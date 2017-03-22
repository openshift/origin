package fake

import (
	v1 "github.com/openshift/origin/pkg/quota/clientset/release_v3_6/typed/quota/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeQuotaV1 struct {
	*core.Fake
}

func (c *FakeQuotaV1) ClusterResourceQuotas() v1.ClusterResourceQuotaInterface {
	return &FakeClusterResourceQuotas{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeQuotaV1) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
