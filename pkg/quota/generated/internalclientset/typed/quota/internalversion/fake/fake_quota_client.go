package fake

import (
	internalversion "github.com/openshift/origin/pkg/quota/generated/internalclientset/typed/quota/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeQuota struct {
	*testing.Fake
}

func (c *FakeQuota) AppliedClusterResourceQuotas(namespace string) internalversion.AppliedClusterResourceQuotaInterface {
	return &FakeAppliedClusterResourceQuotas{c, namespace}
}

func (c *FakeQuota) ClusterResourceQuotas() internalversion.ClusterResourceQuotaInterface {
	return &FakeClusterResourceQuotas{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeQuota) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
