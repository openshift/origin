package fake

import (
	internalversion "github.com/openshift/origin/pkg/route/generated/internalclientset/typed/route/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeRoute struct {
	*testing.Fake
}

func (c *FakeRoute) Routes(namespace string) internalversion.RouteResourceInterface {
	return &FakeRoutes{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeRoute) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
