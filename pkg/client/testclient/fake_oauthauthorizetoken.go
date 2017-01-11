package testclient

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

type FakeOAuthAuthorizeTokens struct {
	Fake *Fake
}

var oAuthAuthorizeTokensResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "oauthauthorizetokens"}

func (c *FakeOAuthAuthorizeTokens) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewRootDeleteAction(oAuthAuthorizeTokensResource, name), &oauthapi.OAuthAuthorizeToken{})
	return err
}

func (c *FakeOAuthAuthorizeTokens) Create(inObj *oauthapi.OAuthAuthorizeToken) (*oauthapi.OAuthAuthorizeToken, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(oAuthAuthorizeTokensResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthAuthorizeToken), err
}
