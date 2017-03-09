package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeRoleBindingRestrictions implements RoleBindingRestrictionInterface. It is
// meant to be embedded into a struct to get a default implementation. This
// makes faking out just the methods you want to test easier.
type FakeRoleBindingRestrictions struct {
	Fake      *Fake
	Namespace string
}

var roleBindingRestritionsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "rolebindingrestrictions"}

func (c *FakeRoleBindingRestrictions) Get(name string) (*authorizationapi.RoleBindingRestriction, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(roleBindingRestritionsResource, c.Namespace, name), &authorizationapi.RoleBindingRestriction{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBindingRestriction), err
}

func (c *FakeRoleBindingRestrictions) List(opts metainternal.ListOptions) (*authorizationapi.RoleBindingRestrictionList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewListAction(roleBindingRestritionsResource, c.Namespace, opts), &authorizationapi.RoleBindingRestrictionList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBindingRestrictionList), err
}

func (c *FakeRoleBindingRestrictions) Create(inObj *authorizationapi.RoleBindingRestriction) (*authorizationapi.RoleBindingRestriction, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(roleBindingRestritionsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBindingRestriction), err
}

func (c *FakeRoleBindingRestrictions) Update(inObj *authorizationapi.RoleBindingRestriction) (*authorizationapi.RoleBindingRestriction, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewUpdateAction(roleBindingRestritionsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBindingRestriction), err
}
func (c *FakeRoleBindingRestrictions) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewDeleteAction(roleBindingRestritionsResource, c.Namespace, name), &authorizationapi.RoleBindingRestriction{})
	return err
}

func (c *FakeRoleBindingRestrictions) Watch(opts metainternal.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewWatchAction(roleBindingRestritionsResource, c.Namespace, opts))
}
