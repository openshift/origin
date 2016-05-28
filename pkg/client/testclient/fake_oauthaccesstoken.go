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

func (c *FakeOAuthAccessTokens) Create(inObj *oauthapi.OAuthAccessToken) (*oauthapi.OAuthAccessToken, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("oauthaccesstokens", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthAccessToken), err
}

// Get returns information about a particular image and error if one occurs.
func (c *FakeOAuthAccessTokens) Get(name string) (*oauthapi.OAuthAccessToken, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("oauthaccesstokens", name), &oauthapi.OAuthAccessToken{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthAccessToken), err
}
