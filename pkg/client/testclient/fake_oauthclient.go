package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/watch"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

type FakeOAuthClient struct {
	Fake *Fake
}

func (c *FakeOAuthClient) Get(name string) (*oauthapi.OAuthClient, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("oauthclients", name), &oauthapi.OAuthClient{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClient), err
}

func (c *FakeOAuthClient) List(opts kapi.ListOptions) (*oauthapi.OAuthClientList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("oauthclients", opts), &oauthapi.OAuthClientList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientList), err
}

func (c *FakeOAuthClient) Create(inObj *oauthapi.OAuthClient) (*oauthapi.OAuthClient, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("oauthclients", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClient), err
}

func (c *FakeOAuthClient) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("oauthclients", name), &oauthapi.OAuthClient{})
	return err
}

func (c *FakeOAuthClient) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewRootWatchAction("oauthclients", opts))
}

func (c *FakeOAuthClient) Update(client *oauthapi.OAuthClient) (*oauthapi.OAuthClient, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootUpdateAction("oauthclients", client), &oauthapi.OAuthClient{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClient), err
}
