package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeRoleBindingRestrictions implements RoleBindingRestrictionInterface. It is
// meant to be embedded into a struct to get a default implementation. This
// makes faking out just the methods you want to test easier.
type FakeRoleBindingRestrictions struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeRoleBindingRestrictions) Get(name string) (*authorizationapi.RoleBindingRestriction, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewGetAction("rolebindingrestrictions", c.Namespace, name), &authorizationapi.RoleBindingRestriction{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBindingRestriction), err
}

func (c *FakeRoleBindingRestrictions) List(opts kapi.ListOptions) (*authorizationapi.RoleBindingRestrictionList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewListAction("rolebindingrestrictions", c.Namespace, opts), &authorizationapi.RoleBindingRestrictionList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBindingRestrictionList), err
}

func (c *FakeRoleBindingRestrictions) Create(inObj *authorizationapi.RoleBindingRestriction) (*authorizationapi.RoleBindingRestriction, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewCreateAction("rolebindingrestrictions", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBindingRestriction), err
}

func (c *FakeRoleBindingRestrictions) Update(inObj *authorizationapi.RoleBindingRestriction) (*authorizationapi.RoleBindingRestriction, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewUpdateAction("rolebindingrestrictions", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBindingRestriction), err
}
func (c *FakeRoleBindingRestrictions) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewDeleteAction("rolebindingrestrictions", c.Namespace, name), &authorizationapi.RoleBindingRestriction{})
	return err
}

func (c *FakeRoleBindingRestrictions) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewWatchAction("rolebindingrestrictions", c.Namespace, opts))
}
