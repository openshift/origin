package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeRoleBindings implements RoleBindingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeRoleBindings struct {
	Fake      *Fake
	Namespace string
}

var roleBindingsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "rolebindings"}

func (c *FakeRoleBindings) Get(name string) (*authorizationapi.RoleBinding, error) {
	obj, err := c.Fake.Invokes(core.NewGetAction(roleBindingsResource, c.Namespace, name), &authorizationapi.RoleBinding{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBinding), err
}

func (c *FakeRoleBindings) List(opts kapi.ListOptions) (*authorizationapi.RoleBindingList, error) {
	obj, err := c.Fake.Invokes(core.NewListAction(roleBindingsResource, c.Namespace, opts), &authorizationapi.RoleBindingList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBindingList), err
}

func (c *FakeRoleBindings) Create(inObj *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(roleBindingsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBinding), err
}

func (c *FakeRoleBindings) Update(inObj *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, error) {
	obj, err := c.Fake.Invokes(core.NewUpdateAction(roleBindingsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBinding), err
}

func (c *FakeRoleBindings) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewDeleteAction(roleBindingsResource, c.Namespace, name), &authorizationapi.RoleBinding{})
	return err
}
