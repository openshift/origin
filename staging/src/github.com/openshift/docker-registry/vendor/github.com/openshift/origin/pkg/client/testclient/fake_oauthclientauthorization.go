package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

type FakeOAuthClientAuthorization struct {
	Fake *Fake
}

var oAuthClientAuthorizationsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "oauthclientauthorizations"}
var oAuthClientAuthorizationsKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "OAuthClientAuthorization"}

func (c *FakeOAuthClientAuthorization) Get(name string, options metav1.GetOptions) (*oauthapi.OAuthClientAuthorization, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(oAuthClientAuthorizationsResource, name), &oauthapi.OAuthClientAuthorization{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientAuthorization), err
}

func (c *FakeOAuthClientAuthorization) List(opts metav1.ListOptions) (*oauthapi.OAuthClientAuthorizationList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(oAuthClientAuthorizationsResource, oAuthClientAuthorizationsKind, opts), &oauthapi.OAuthClientAuthorizationList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientAuthorizationList), err
}

func (c *FakeOAuthClientAuthorization) Create(inObj *oauthapi.OAuthClientAuthorization) (*oauthapi.OAuthClientAuthorization, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(oAuthClientAuthorizationsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientAuthorization), err
}

func (c *FakeOAuthClientAuthorization) Update(inObj *oauthapi.OAuthClientAuthorization) (*oauthapi.OAuthClientAuthorization, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(oAuthClientAuthorizationsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientAuthorization), err
}

func (c *FakeOAuthClientAuthorization) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(oAuthClientAuthorizationsResource, name), &oauthapi.OAuthClientAuthorization{})
	return err
}

func (c *FakeOAuthClientAuthorization) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewRootWatchAction(oAuthClientAuthorizationsResource, opts))
}
