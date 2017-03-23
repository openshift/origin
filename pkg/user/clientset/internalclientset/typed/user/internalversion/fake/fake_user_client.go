package fake

import (
	internalversion "github.com/openshift/origin/pkg/user/clientset/internalclientset/typed/user/internalversion"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeUser struct {
	*core.Fake
}

func (c *FakeUser) Users(namespace string) internalversion.UserResourceInterface {
	return &FakeUsers{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeUser) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
