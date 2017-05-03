package fake

import (
	internalversion "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeOauth struct {
	*testing.Fake
}

func (c *FakeOauth) OAuthClients(namespace string) internalversion.OAuthClientInterface {
	return &FakeOAuthClients{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeOauth) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
