package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/watch"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

type FakeOAuthClientAuthorization struct {
	Fake *Fake
}

func (c *FakeOAuthClientAuthorization) Get(name string) (*oauthapi.OAuthClientAuthorization, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("oauthClientAuthorizations", name), &oauthapi.OAuthClientAuthorization{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientAuthorization), err
}

func (c *FakeOAuthClientAuthorization) List(opts kapi.ListOptions) (*oauthapi.OAuthClientAuthorizationList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("oauthClientAuthorizations", opts), &oauthapi.OAuthClientAuthorizationList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientAuthorizationList), err
}

func (c *FakeOAuthClientAuthorization) Create(inObj *oauthapi.OAuthClientAuthorization) (*oauthapi.OAuthClientAuthorization, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("oauthClientAuthorizations", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientAuthorization), err
}

func (c *FakeOAuthClientAuthorization) Update(inObj *oauthapi.OAuthClientAuthorization) (*oauthapi.OAuthClientAuthorization, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootUpdateAction("oauthClientAuthorizations", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientAuthorization), err
}

func (c *FakeOAuthClientAuthorization) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("oauthClientAuthorizations", name), &oauthapi.OAuthClientAuthorization{})
	return err
}

func (c *FakeOAuthClientAuthorization) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewRootWatchAction("oauthClientAuthorizations", opts))
}
