package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// FakeRoles implements RoleInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeRoles struct {
	Fake      *Fake
	Namespace string
}

var rolesResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "roles"}
var rolesKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "Role"}

func (c *FakeRoles) Get(name string, options metav1.GetOptions) (*authorizationapi.Role, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(rolesResource, c.Namespace, name), &authorizationapi.Role{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.Role), err
}

func (c *FakeRoles) List(opts metav1.ListOptions) (*authorizationapi.RoleList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewListAction(rolesResource, rolesKind, c.Namespace, opts), &authorizationapi.RoleList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.RoleList), err
}

func (c *FakeRoles) Create(inObj *authorizationapi.Role) (*authorizationapi.Role, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(rolesResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.Role), err
}

func (c *FakeRoles) Update(inObj *authorizationapi.Role) (*authorizationapi.Role, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewUpdateAction(rolesResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.Role), err
}

func (c *FakeRoles) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewDeleteAction(rolesResource, c.Namespace, name), &authorizationapi.Role{})
	return err
}
