package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

type FakeOAuthAuthorizeTokens struct {
	Fake *Fake
}

var oAuthAuthorizeTokensResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "oauthauthorizetokens"}

func (c *FakeOAuthAuthorizeTokens) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(oAuthAuthorizeTokensResource, name), &oauthapi.OAuthAuthorizeToken{})
	return err
}

func (c *FakeOAuthAuthorizeTokens) Create(inObj *oauthapi.OAuthAuthorizeToken) (*oauthapi.OAuthAuthorizeToken, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(oAuthAuthorizeTokensResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthAuthorizeToken), err
}

func (c *FakeOAuthAuthorizeTokens) Get(name string, options metav1.GetOptions) (*oauthapi.OAuthAuthorizeToken, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(oAuthAuthorizeTokensResource, name), &oauthapi.OAuthAuthorizeToken{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthAuthorizeToken), err
}
