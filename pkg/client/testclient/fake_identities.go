package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeIdentities implements IdentitiesInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeIdentities struct {
	Fake *Fake
}

var identitiesResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "identities"}

func (c *FakeIdentities) Get(name string) (*userapi.Identity, error) {
	obj, err := c.Fake.Invokes(core.NewRootGetAction(identitiesResource, name), &userapi.Identity{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Identity), err
}

func (c *FakeIdentities) List(opts kapi.ListOptions) (*userapi.IdentityList, error) {
	obj, err := c.Fake.Invokes(core.NewRootListAction(identitiesResource, opts), &userapi.IdentityList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.IdentityList), err
}

func (c *FakeIdentities) Create(inObj *userapi.Identity) (*userapi.Identity, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(identitiesResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Identity), err
}

func (c *FakeIdentities) Update(inObj *userapi.Identity) (*userapi.Identity, error) {
	obj, err := c.Fake.Invokes(core.NewRootUpdateAction(identitiesResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Identity), err
}

func (c *FakeIdentities) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewRootDeleteAction(identitiesResource, name), nil)
	return err
}
