package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeUserIdentityMappings implements UserIdentityMappingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeUserIdentityMappings struct {
	Fake *Fake
}

func (c *FakeUserIdentityMappings) Get(name string) (*userapi.UserIdentityMapping, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("useridentitymappings", name), &userapi.UserIdentityMapping{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.UserIdentityMapping), err
}

func (c *FakeUserIdentityMappings) Create(inObj *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("useridentitymappings", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.UserIdentityMapping), err
}

func (c *FakeUserIdentityMappings) Update(inObj *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootUpdateAction("useridentitymappings", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.UserIdentityMapping), err
}

func (c *FakeUserIdentityMappings) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("useridentitymappings", name), &userapi.UserIdentityMapping{})
	return err
}
