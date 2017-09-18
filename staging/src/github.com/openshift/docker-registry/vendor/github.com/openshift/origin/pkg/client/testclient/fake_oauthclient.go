package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

type FakeOAuthClient struct {
	Fake *Fake
}

var oAuthClientsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "oauthclients"}
var oAuthClientsKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "OAuthClient"}

func (c *FakeOAuthClient) Get(name string, options metav1.GetOptions) (*oauthapi.OAuthClient, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(oAuthClientsResource, name), &oauthapi.OAuthClient{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClient), err
}

func (c *FakeOAuthClient) List(opts metav1.ListOptions) (*oauthapi.OAuthClientList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(oAuthClientsResource, oAuthClientsKind, opts), &oauthapi.OAuthClientList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientList), err
}

func (c *FakeOAuthClient) Create(inObj *oauthapi.OAuthClient) (*oauthapi.OAuthClient, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(oAuthClientsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClient), err
}

func (c *FakeOAuthClient) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(oAuthClientsResource, name), &oauthapi.OAuthClient{})
	return err
}

func (c *FakeOAuthClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewRootWatchAction(oAuthClientsResource, opts))
}

func (c *FakeOAuthClient) Update(client *oauthapi.OAuthClient) (*oauthapi.OAuthClient, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(oAuthClientsResource, client), &oauthapi.OAuthClient{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClient), err
}
