package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeRoles implements RoleInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeRoles struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeRoles) Get(name string) (*authorizationapi.Role, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewGetAction("roles", c.Namespace, name), &authorizationapi.Role{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.Role), err
}

func (c *FakeRoles) List(opts kapi.ListOptions) (*authorizationapi.RoleList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewListAction("roles", c.Namespace, opts), &authorizationapi.RoleList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleList), err
}

func (c *FakeRoles) Create(inObj *authorizationapi.Role) (*authorizationapi.Role, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewCreateAction("roles", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.Role), err
}

func (c *FakeRoles) Update(inObj *authorizationapi.Role) (*authorizationapi.Role, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewUpdateAction("roles", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.Role), err
}

func (c *FakeRoles) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewDeleteAction("roles", c.Namespace, name), &authorizationapi.Role{})
	return err
}
