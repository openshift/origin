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

// FakeRoleBindingRestrictions implements RoleBindingRestrictionInterface
type FakeRoleBindingRestrictions struct {
	Fake *FakeAuthorizationV1
	ns   string
}

var rolebindingrestrictionsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "v1", Resource: "rolebindingrestrictions"}

var rolebindingrestrictionsKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "v1", Kind: "RoleBindingRestriction"}

// Get takes name of the roleBindingRestriction, and returns the corresponding roleBindingRestriction object, and an error if there is any.
func (c *FakeRoleBindingRestrictions) Get(name string, options v1.GetOptions) (result *authorization_v1.RoleBindingRestriction, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(rolebindingrestrictionsResource, c.ns, name), &authorization_v1.RoleBindingRestriction{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.RoleBindingRestriction), err
}

// List takes label and field selectors, and returns the list of RoleBindingRestrictions that match those selectors.
func (c *FakeRoleBindingRestrictions) List(opts v1.ListOptions) (result *authorization_v1.RoleBindingRestrictionList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(rolebindingrestrictionsResource, rolebindingrestrictionsKind, c.ns, opts), &authorization_v1.RoleBindingRestrictionList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &authorization_v1.RoleBindingRestrictionList{}
	for _, item := range obj.(*authorization_v1.RoleBindingRestrictionList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested roleBindingRestrictions.
func (c *FakeRoleBindingRestrictions) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(rolebindingrestrictionsResource, c.ns, opts))

}

// Create takes the representation of a roleBindingRestriction and creates it.  Returns the server's representation of the roleBindingRestriction, and an error, if there is any.
func (c *FakeRoleBindingRestrictions) Create(roleBindingRestriction *authorization_v1.RoleBindingRestriction) (result *authorization_v1.RoleBindingRestriction, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(rolebindingrestrictionsResource, c.ns, roleBindingRestriction), &authorization_v1.RoleBindingRestriction{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.RoleBindingRestriction), err
}

// Update takes the representation of a roleBindingRestriction and updates it. Returns the server's representation of the roleBindingRestriction, and an error, if there is any.
func (c *FakeRoleBindingRestrictions) Update(roleBindingRestriction *authorization_v1.RoleBindingRestriction) (result *authorization_v1.RoleBindingRestriction, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(rolebindingrestrictionsResource, c.ns, roleBindingRestriction), &authorization_v1.RoleBindingRestriction{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.RoleBindingRestriction), err
}

// Delete takes name of the roleBindingRestriction and deletes it. Returns an error if one occurs.
func (c *FakeRoleBindingRestrictions) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(rolebindingrestrictionsResource, c.ns, name), &authorization_v1.RoleBindingRestriction{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeRoleBindingRestrictions) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(rolebindingrestrictionsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &authorization_v1.RoleBindingRestrictionList{})
	return err
}

// Patch applies the patch and returns the patched roleBindingRestriction.
func (c *FakeRoleBindingRestrictions) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization_v1.RoleBindingRestriction, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(rolebindingrestrictionsResource, c.ns, name, data, subresources...), &authorization_v1.RoleBindingRestriction{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.RoleBindingRestriction), err
}
