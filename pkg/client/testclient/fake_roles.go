package testclient

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeRoles implements RoleInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeRoles struct {
	Fake *Fake
}

func (c *FakeRoles) List(label labels.Selector, field fields.Selector) (*authorizationapi.RoleList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-role"}, &authorizationapi.RoleList{})
	return obj.(*authorizationapi.RoleList), err
}

func (c *FakeRoles) Get(name string) (*authorizationapi.Role, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-role"}, &authorizationapi.Role{})
	return obj.(*authorizationapi.Role), err
}

func (c *FakeRoles) Create(role *authorizationapi.Role) (*authorizationapi.Role, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-role", Value: role}, &authorizationapi.Role{})
	return obj.(*authorizationapi.Role), err
}

func (c *FakeRoles) Update(role *authorizationapi.Role) (*authorizationapi.Role, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "update-role"}, &authorizationapi.Role{})
	return obj.(*authorizationapi.Role), err
}

func (c *FakeRoles) Delete(name string) error {
	c.Fake.Lock.Lock()
	defer c.Fake.Lock.Unlock()

	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-role", Value: name})
	return nil
}
