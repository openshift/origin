package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeIdentities implements IdentitiesInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeIdentities struct {
	Fake *Fake
}

func (c *FakeIdentities) Get(name string) (*userapi.Identity, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("identities", name), &userapi.Identity{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Identity), err
}

func (c *FakeIdentities) List(opts kapi.ListOptions) (*userapi.IdentityList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("identities", opts), &userapi.IdentityList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.IdentityList), err
}

func (c *FakeIdentities) Create(inObj *userapi.Identity) (*userapi.Identity, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("identities", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Identity), err
}

func (c *FakeIdentities) Update(inObj *userapi.Identity) (*userapi.Identity, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootUpdateAction("identities", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Identity), err
}

func (c *FakeIdentities) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("identities", name), nil)
	return err
}
