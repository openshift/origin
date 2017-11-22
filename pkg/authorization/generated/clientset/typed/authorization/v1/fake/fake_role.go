package fake

import (
	authorization_v1 "github.com/openshift/api/authorization/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeRoles implements RoleInterface
type FakeRoles struct {
	Fake *FakeAuthorizationV1
	ns   string
}

var rolesResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "v1", Resource: "roles"}

var rolesKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "v1", Kind: "Role"}

// Get takes name of the role, and returns the corresponding role object, and an error if there is any.
func (c *FakeRoles) Get(name string, options v1.GetOptions) (result *authorization_v1.Role, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(rolesResource, c.ns, name), &authorization_v1.Role{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.Role), err
}

// List takes label and field selectors, and returns the list of Roles that match those selectors.
func (c *FakeRoles) List(opts v1.ListOptions) (result *authorization_v1.RoleList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(rolesResource, rolesKind, c.ns, opts), &authorization_v1.RoleList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &authorization_v1.RoleList{}
	for _, item := range obj.(*authorization_v1.RoleList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested roles.
func (c *FakeRoles) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(rolesResource, c.ns, opts))

}

// Create takes the representation of a role and creates it.  Returns the server's representation of the role, and an error, if there is any.
func (c *FakeRoles) Create(role *authorization_v1.Role) (result *authorization_v1.Role, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(rolesResource, c.ns, role), &authorization_v1.Role{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.Role), err
}

// Update takes the representation of a role and updates it. Returns the server's representation of the role, and an error, if there is any.
func (c *FakeRoles) Update(role *authorization_v1.Role) (result *authorization_v1.Role, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(rolesResource, c.ns, role), &authorization_v1.Role{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.Role), err
}

// Delete takes name of the role and deletes it. Returns an error if one occurs.
func (c *FakeRoles) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(rolesResource, c.ns, name), &authorization_v1.Role{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeRoles) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(rolesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &authorization_v1.RoleList{})
	return err
}

// Patch applies the patch and returns the patched role.
func (c *FakeRoles) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization_v1.Role, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(rolesResource, c.ns, name, data, subresources...), &authorization_v1.Role{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.Role), err
}
