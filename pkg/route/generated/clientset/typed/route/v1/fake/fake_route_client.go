package fake

import (
	v1 "github.com/openshift/origin/pkg/route/generated/clientset/typed/route/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeRouteV1 struct {
	*testing.Fake
}

func (c *FakeRouteV1) Routes(namespace string) v1.RouteResourceInterface {
	return &FakeRoutes{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeRouteV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
