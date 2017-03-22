package fake

import (
	internalversion "github.com/openshift/origin/pkg/oauth/clientset/internalclientset/typed/oauth/internalversion"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeOauth struct {
	*core.Fake
}

func (c *FakeOauth) OAuthClients(namespace string) internalversion.OAuthClientInterface {
	return &FakeOAuthClients{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeOauth) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
