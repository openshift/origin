package fake

import (
	internalversion "github.com/openshift/origin/pkg/route/clientset/internalclientset/typed/route/internalversion"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeRoute struct {
	*core.Fake
}

func (c *FakeRoute) Routes(namespace string) internalversion.RouteResourceInterface {
	return &FakeRoutes{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeRoute) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
