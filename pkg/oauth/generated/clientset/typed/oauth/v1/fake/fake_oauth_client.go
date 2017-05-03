package fake

import (
	v1 "github.com/openshift/origin/pkg/oauth/generated/clientset/typed/oauth/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeOauthV1 struct {
	*testing.Fake
}

func (c *FakeOauthV1) OAuthClients(namespace string) v1.OAuthClientInterface {
	return &FakeOAuthClients{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeOauthV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
