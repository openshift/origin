package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

type FakeOAuthAuthorizeTokens struct {
	Fake *Fake
}

func (c *FakeOAuthAuthorizeTokens) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("oauthauthorizetokens", name), &oauthapi.OAuthAuthorizeToken{})
	return err
}

func (c *FakeOAuthAuthorizeTokens) Create(inObj *oauthapi.OAuthAuthorizeToken) (*oauthapi.OAuthAuthorizeToken, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("oauthauthorizetokens", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthAuthorizeToken), err
}
