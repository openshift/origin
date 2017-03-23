package fake

import (
	v1 "github.com/openshift/origin/pkg/oauth/clientset/release_v3_6/typed/oauth/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeOauthV1 struct {
	*core.Fake
}

func (c *FakeOauthV1) OAuthClients(namespace string) v1.OAuthClientInterface {
	return &FakeOAuthClients{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeOauthV1) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
