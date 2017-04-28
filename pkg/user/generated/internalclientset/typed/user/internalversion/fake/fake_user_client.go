package fake

import (
	internalversion "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeUser struct {
	*testing.Fake
}

func (c *FakeUser) Users(namespace string) internalversion.UserResourceInterface {
	return &FakeUsers{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeUser) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
