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

// FakeRoleBindings implements RoleBindingInterface
type FakeRoleBindings struct {
	Fake *FakeAuthorizationV1
	ns   string
}

var rolebindingsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "v1", Resource: "rolebindings"}

var rolebindingsKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "v1", Kind: "RoleBinding"}

// Get takes name of the roleBinding, and returns the corresponding roleBinding object, and an error if there is any.
func (c *FakeRoleBindings) Get(name string, options v1.GetOptions) (result *authorization_v1.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(rolebindingsResource, c.ns, name), &authorization_v1.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.RoleBinding), err
}

// List takes label and field selectors, and returns the list of RoleBindings that match those selectors.
func (c *FakeRoleBindings) List(opts v1.ListOptions) (result *authorization_v1.RoleBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(rolebindingsResource, rolebindingsKind, c.ns, opts), &authorization_v1.RoleBindingList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &authorization_v1.RoleBindingList{}
	for _, item := range obj.(*authorization_v1.RoleBindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested roleBindings.
func (c *FakeRoleBindings) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(rolebindingsResource, c.ns, opts))

}

// Create takes the representation of a roleBinding and creates it.  Returns the server's representation of the roleBinding, and an error, if there is any.
func (c *FakeRoleBindings) Create(roleBinding *authorization_v1.RoleBinding) (result *authorization_v1.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(rolebindingsResource, c.ns, roleBinding), &authorization_v1.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.RoleBinding), err
}

// Update takes the representation of a roleBinding and updates it. Returns the server's representation of the roleBinding, and an error, if there is any.
func (c *FakeRoleBindings) Update(roleBinding *authorization_v1.RoleBinding) (result *authorization_v1.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(rolebindingsResource, c.ns, roleBinding), &authorization_v1.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.RoleBinding), err
}

// Delete takes name of the roleBinding and deletes it. Returns an error if one occurs.
func (c *FakeRoleBindings) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(rolebindingsResource, c.ns, name), &authorization_v1.RoleBinding{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeRoleBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(rolebindingsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &authorization_v1.RoleBindingList{})
	return err
}

// Patch applies the patch and returns the patched roleBinding.
func (c *FakeRoleBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization_v1.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(rolebindingsResource, c.ns, name, data, subresources...), &authorization_v1.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.RoleBinding), err
}
