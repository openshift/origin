package fake

import (
	api "github.com/openshift/origin/pkg/authorization/api"
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

func (c *FakeRoleBindings) Create(roleBinding *api.RoleBinding) (result *api.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(rolebindingsResource, c.ns, roleBinding), &api.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.RoleBinding), err
}

func (c *FakeRoleBindings) Update(roleBinding *api.RoleBinding) (result *api.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(rolebindingsResource, c.ns, roleBinding), &api.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.RoleBinding), err
}

func (c *FakeRoleBindings) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(rolebindingsResource, c.ns, name), &api.RoleBinding{})

	return err
}

func (c *FakeRoleBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(rolebindingsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.RoleBindingList{})
	return err
}

func (c *FakeRoleBindings) Get(name string, options v1.GetOptions) (result *api.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(rolebindingsResource, c.ns, name), &api.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.RoleBinding), err
}

func (c *FakeRoleBindings) List(opts v1.ListOptions) (result *api.RoleBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(rolebindingsResource, c.ns, opts), &api.RoleBindingList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.RoleBindingList{}
	for _, item := range obj.(*api.RoleBindingList).Items {
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
func (c *FakeRoleBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(rolebindingsResource, c.ns, name, data, subresources...), &api.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.RoleBinding), err
}
