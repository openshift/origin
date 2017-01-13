package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/watch"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

type FakeSelfOAuthClientAuthorization struct {
	Fake *Fake
}

var selfOAuthClientAuthorizationsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "selfoauthclientauthorizations"}

func (c *FakeSelfOAuthClientAuthorization) Get(name string) (*oauthapi.SelfOAuthClientAuthorization, error) {
	obj, err := c.Fake.Invokes(core.NewRootGetAction(selfOAuthClientAuthorizationsResource, name), &oauthapi.SelfOAuthClientAuthorization{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.SelfOAuthClientAuthorization), err
}

func (c *FakeSelfOAuthClientAuthorization) List(opts kapi.ListOptions) (*oauthapi.SelfOAuthClientAuthorizationList, error) {
	obj, err := c.Fake.Invokes(core.NewRootListAction(selfOAuthClientAuthorizationsResource, opts), &oauthapi.SelfOAuthClientAuthorizationList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.SelfOAuthClientAuthorizationList), err
}

func (c *FakeSelfOAuthClientAuthorization) Create(inObj *oauthapi.SelfOAuthClientAuthorization) (*oauthapi.SelfOAuthClientAuthorization, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(selfOAuthClientAuthorizationsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.SelfOAuthClientAuthorization), err
}

func (c *FakeSelfOAuthClientAuthorization) Update(inObj *oauthapi.SelfOAuthClientAuthorization) (*oauthapi.SelfOAuthClientAuthorization, error) {
	obj, err := c.Fake.Invokes(core.NewRootUpdateAction(selfOAuthClientAuthorizationsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.SelfOAuthClientAuthorization), err
}

func (c *FakeSelfOAuthClientAuthorization) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewRootDeleteAction(selfOAuthClientAuthorizationsResource, name), &oauthapi.SelfOAuthClientAuthorization{})
	return err
}

func (c *FakeSelfOAuthClientAuthorization) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(core.NewRootWatchAction(selfOAuthClientAuthorizationsResource, opts))
}
