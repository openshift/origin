package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	core "k8s.io/client-go/testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeRoleBindings implements RoleBindingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeRoleBindings struct {
	Fake      *Fake
	Namespace string
}

var roleBindingsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "rolebindings"}

func (c *FakeRoleBindings) Get(name string, options metav1.GetOptions) (*authorizationapi.RoleBinding, error) {
	obj, err := c.Fake.Invokes(core.NewGetAction(roleBindingsResource, c.Namespace, name), &authorizationapi.RoleBinding{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleBinding), err
}

func (c *FakeRoleBindings) List(opts metainternal.ListOptions) (*authorizationapi.RoleBindingList, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	obj, err := c.Fake.Invokes(core.NewListAction(roleBindingsResource, c.Namespace, optsv1), &authorizationapi.RoleBindingList{})
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
