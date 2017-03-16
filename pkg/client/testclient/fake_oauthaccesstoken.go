package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

// FakeOAuthAccessTokens implements OAuthAccessTokenInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeOAuthAccessTokens struct {
	Fake *Fake
}

var oAuthAccessTokensResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "oauthaccesstokens"}

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

func (c *FakeOAuthAccessTokens) List(opts metainternal.ListOptions) (*oauthapi.OAuthAccessTokenList, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(oAuthAccessTokensResource, optsv1), &oauthapi.OAuthAccessTokenList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthAccessTokenList), err
}
