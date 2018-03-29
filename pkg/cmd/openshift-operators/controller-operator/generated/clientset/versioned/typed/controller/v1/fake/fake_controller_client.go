package fake

import (
	v1 "github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/generated/clientset/versioned/typed/controller/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeControllerV1 struct {
	*testing.Fake
}

func (c *FakeControllerV1) OpenShiftControllerConfigs() v1.OpenShiftControllerConfigInterface {
	return &FakeOpenShiftControllerConfigs{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeControllerV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
