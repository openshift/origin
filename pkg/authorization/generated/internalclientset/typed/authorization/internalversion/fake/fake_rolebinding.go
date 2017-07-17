package fake

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeRoleBindings implements RoleBindingInterface
type FakeRoleBindings struct {
	Fake *FakeAuthorization
	ns   string
}

var rolebindingsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "rolebindings"}

var rolebindingsKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "", Kind: "RoleBinding"}

func (c *FakeRoleBindings) Create(roleBinding *authorization.RoleBinding) (result *authorization.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(rolebindingsResource, c.ns, roleBinding), &authorization.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.RoleBinding), err
}

func (c *FakeRoleBindings) Update(roleBinding *authorization.RoleBinding) (result *authorization.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(rolebindingsResource, c.ns, roleBinding), &authorization.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.RoleBinding), err
}

func (c *FakeRoleBindings) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(rolebindingsResource, c.ns, name), &authorization.RoleBinding{})

	return err
}

func (c *FakeRoleBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(rolebindingsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &authorization.RoleBindingList{})
	return err
}

func (c *FakeRoleBindings) Get(name string, options v1.GetOptions) (result *authorization.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(rolebindingsResource, c.ns, name), &authorization.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.RoleBinding), err
}

func (c *FakeRoleBindings) List(opts v1.ListOptions) (result *authorization.RoleBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(rolebindingsResource, rolebindingsKind, c.ns, opts), &authorization.RoleBindingList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &authorization.RoleBindingList{}
	for _, item := range obj.(*authorization.RoleBindingList).Items {
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

// Patch applies the patch and returns the patched roleBinding.
func (c *FakeRoleBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(rolebindingsResource, c.ns, name, data, subresources...), &authorization.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.RoleBinding), err
}
