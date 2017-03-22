package fake

import (
	v1 "github.com/openshift/origin/pkg/route/clientset/release_v3_6/typed/route/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeRouteV1 struct {
	*core.Fake
}

func (c *FakeRouteV1) Routes(namespace string) v1.RouteResourceInterface {
	return &FakeRoutes{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeRouteV1) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
