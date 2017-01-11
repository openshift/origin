package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/watch"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

type FakeOAuthClientAuthorization struct {
	Fake *Fake
}

var oAuthClientAuthorizationsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "oauthclientauthorizations"}

func (c *FakeOAuthClientAuthorization) Get(name string) (*oauthapi.OAuthClientAuthorization, error) {
	obj, err := c.Fake.Invokes(core.NewRootGetAction(oAuthClientAuthorizationsResource, name), &oauthapi.OAuthClientAuthorization{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientAuthorization), err
}

func (c *FakeOAuthClientAuthorization) List(opts kapi.ListOptions) (*oauthapi.OAuthClientAuthorizationList, error) {
	obj, err := c.Fake.Invokes(core.NewRootListAction(oAuthClientAuthorizationsResource, opts), &oauthapi.OAuthClientAuthorizationList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientAuthorizationList), err
}

func (c *FakeOAuthClientAuthorization) Create(inObj *oauthapi.OAuthClientAuthorization) (*oauthapi.OAuthClientAuthorization, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(oAuthClientAuthorizationsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientAuthorization), err
}

func (c *FakeOAuthClientAuthorization) Update(inObj *oauthapi.OAuthClientAuthorization) (*oauthapi.OAuthClientAuthorization, error) {
	obj, err := c.Fake.Invokes(core.NewRootUpdateAction(oAuthClientAuthorizationsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientAuthorization), err
}

func (c *FakeOAuthClientAuthorization) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewRootDeleteAction(oAuthClientAuthorizationsResource, name), &oauthapi.OAuthClientAuthorization{})
	return err
}

func (c *FakeOAuthClientAuthorization) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(core.NewRootWatchAction(oAuthClientAuthorizationsResource, opts))
}
