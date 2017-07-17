package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// FakeRoleBindings implements RoleBindingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeRoleBindings struct {
	Fake      *Fake
	Namespace string
}

var roleBindingsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "rolebindings"}
var roleBindingsKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "RoleBinding"}

func (c *FakeRoleBindings) Get(name string, options metav1.GetOptions) (*authorizationapi.RoleBinding, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(roleBindingsResource, c.Namespace, name), &authorizationapi.RoleBinding{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBinding), err
}

func (c *FakeRoleBindings) List(opts metav1.ListOptions) (*authorizationapi.RoleBindingList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewListAction(roleBindingsResource, roleBindingsKind, c.Namespace, opts), &authorizationapi.RoleBindingList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBindingList), err
}

func (c *FakeRoleBindings) Create(inObj *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(roleBindingsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBinding), err
}

func (c *FakeRoleBindings) Update(inObj *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewUpdateAction(roleBindingsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBinding), err
}

func (c *FakeRoleBindings) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewDeleteAction(roleBindingsResource, c.Namespace, name), &authorizationapi.RoleBinding{})
	return err
}
