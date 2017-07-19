package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

// FakeOAuthAccessTokens implements OAuthAccessTokenInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeOAuthAccessTokens struct {
	Fake *Fake
}

var oAuthAccessTokensResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "oauthaccesstokens"}
var oAuthAccessTokensKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "OAuthAccessToken"}

func (c *FakeOAuthAccessTokens) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(oAuthAccessTokensResource, name), &oauthapi.OAuthAccessToken{})
	return err
}

func (c *FakeOAuthAccessTokens) Create(inObj *oauthapi.OAuthAccessToken) (*oauthapi.OAuthAccessToken, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(oAuthAccessTokensResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthAccessToken), err
}

// Get returns information about a particular image and error if one occurs.
func (c *FakeOAuthAccessTokens) Get(name string, options metav1.GetOptions) (*oauthapi.OAuthAccessToken, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(oAuthAccessTokensResource, name), &oauthapi.OAuthAccessToken{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthAccessToken), err
}

func (c *FakeOAuthAccessTokens) List(opts metav1.ListOptions) (*oauthapi.OAuthAccessTokenList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(oAuthAccessTokensResource, oAuthAccessTokensKind, opts), &oauthapi.OAuthAccessTokenList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthAccessTokenList), err
}
