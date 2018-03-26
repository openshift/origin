package fake

import (
	v1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/generated/clientset/versioned/typed/apiserver/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeApiserverV1 struct {
	*testing.Fake
}

func (c *FakeApiserverV1) OpenShiftAPIServerConfigs() v1.OpenShiftAPIServerConfigInterface {
	return &FakeOpenShiftAPIServerConfigs{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeApiserverV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
