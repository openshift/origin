package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

// FakeOAuthAccessTokens implements OAuthAccessTokenInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeOAuthAccessTokens struct {
	Fake *Fake
}

func (c *FakeOAuthAccessTokens) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("oauthaccesstokens", name), &oauthapi.OAuthAccessToken{})
	return err
}
