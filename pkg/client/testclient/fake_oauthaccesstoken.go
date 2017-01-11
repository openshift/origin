package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

// FakeOAuthAccessTokens implements OAuthAccessTokenInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeOAuthAccessTokens struct {
	Fake *Fake
}

var oAuthAccessTokensResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "oauthaccesstokens"}

func (c *FakeOAuthAccessTokens) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewRootDeleteAction(oAuthAccessTokensResource, name), &oauthapi.OAuthAccessToken{})
	return err
}

func (c *FakeOAuthAccessTokens) Create(inObj *oauthapi.OAuthAccessToken) (*oauthapi.OAuthAccessToken, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(oAuthAccessTokensResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthAccessToken), err
}

// Get returns information about a particular image and error if one occurs.
func (c *FakeOAuthAccessTokens) Get(name string) (*oauthapi.OAuthAccessToken, error) {
	obj, err := c.Fake.Invokes(core.NewRootGetAction(oAuthAccessTokensResource, name), &oauthapi.OAuthAccessToken{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthAccessToken), err
}

func (c *FakeOAuthAccessTokens) List(opts kapi.ListOptions) (*oauthapi.OAuthAccessTokenList, error) {
	obj, err := c.Fake.Invokes(core.NewRootListAction(oAuthAccessTokensResource, opts), &oauthapi.OAuthAccessTokenList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthAccessTokenList), err
}
