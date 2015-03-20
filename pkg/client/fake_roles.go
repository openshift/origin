package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeRoles struct {
	Fake *Fake
}

func (c *FakeRoles) List(label labels.Selector, field fields.Selector) (*authorizationapi.RoleList, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "list-role"})
	return &authorizationapi.RoleList{}, nil
}

func (c *FakeRoles) Get(name string) (*authorizationapi.Role, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-role"})
	return &authorizationapi.Role{}, nil
}

func (c *FakeRoles) Create(role *authorizationapi.Role) (*authorizationapi.Role, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-role", Value: role})
	return &authorizationapi.Role{}, nil
}

func (c *FakeRoles) Update(role *authorizationapi.Role) (*authorizationapi.Role, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "update-role"})
	return &authorizationapi.Role{}, nil
}

func (c *FakeRoles) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-role", Value: name})
	return nil
}
