package fake

import (
	v1 "github.com/openshift/origin/pkg/authorization/generated/clientset/typed/authorization/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeAuthorizationV1 struct {
	*testing.Fake
}

func (c *FakeAuthorizationV1) Policies(namespace string) v1.PolicyInterface {
	return &FakePolicies{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeAuthorizationV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
