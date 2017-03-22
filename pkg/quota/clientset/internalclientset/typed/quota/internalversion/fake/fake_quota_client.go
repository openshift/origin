package fake

import (
	internalversion "github.com/openshift/origin/pkg/quota/clientset/internalclientset/typed/quota/internalversion"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeQuota struct {
	*core.Fake
}

func (c *FakeQuota) ClusterResourceQuotas() internalversion.ClusterResourceQuotaInterface {
	return &FakeClusterResourceQuotas{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeQuota) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
