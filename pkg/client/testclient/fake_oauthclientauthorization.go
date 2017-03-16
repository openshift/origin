package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

type FakeOAuthClientAuthorization struct {
	Fake *Fake
}

var oAuthClientAuthorizationsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "oauthclientauthorizations"}

func (c *FakeOAuthClientAuthorization) Get(name string, options metav1.GetOptions) (*oauthapi.OAuthClientAuthorization, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(oAuthClientAuthorizationsResource, name), &oauthapi.OAuthClientAuthorization{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientAuthorization), err
}

func (c *FakeOAuthClientAuthorization) List(opts metainternal.ListOptions) (*oauthapi.OAuthClientAuthorizationList, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(oAuthClientAuthorizationsResource, optsv1), &oauthapi.OAuthClientAuthorizationList{})
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

func (c *FakeOAuthClientAuthorization) Watch(opts metainternal.ListOptions) (watch.Interface, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	return c.Fake.InvokesWatch(clientgotesting.NewRootWatchAction(oAuthClientAuthorizationsResource, optsv1))
}
