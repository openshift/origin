package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeRoleBindings struct {
	Fake *Fake
}

func (c *FakeRoleBindings) List(label labels.Selector, field fields.Selector) (*authorizationapi.RoleBindingList, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "list-roleBinding"})
	return &authorizationapi.RoleBindingList{}, nil
}

func (c *FakeRoleBindings) Get(name string) (*authorizationapi.RoleBinding, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-roleBinding"})
	return &authorizationapi.RoleBinding{}, nil
}

func (c *FakeRoleBindings) Create(roleBinding *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-roleBinding", Value: roleBinding})
	return &authorizationapi.RoleBinding{}, nil
}

func (c *FakeRoleBindings) Update(roleBinding *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "update-roleBinding"})
	return &authorizationapi.RoleBinding{}, nil
}

func (c *FakeRoleBindings) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-roleBinding", Value: name})
	return nil
}
