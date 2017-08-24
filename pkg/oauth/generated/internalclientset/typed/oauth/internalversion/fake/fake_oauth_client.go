package fake

import (
	internalversion "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeOauth struct {
	*testing.Fake
}

func (c *FakeOauth) OAuthAccessTokens() internalversion.OAuthAccessTokenInterface {
	return &FakeOAuthAccessTokens{c}
}

func (c *FakeOauth) OAuthAuthorizeTokens() internalversion.OAuthAuthorizeTokenInterface {
	return &FakeOAuthAuthorizeTokens{c}
}

func (c *FakeOauth) OAuthClients() internalversion.OAuthClientInterface {
	return &FakeOAuthClients{c}
}

func (c *FakeOauth) OAuthClientAuthorizations() internalversion.OAuthClientAuthorizationInterface {
	return &FakeOAuthClientAuthorizations{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeOauth) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
