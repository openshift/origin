package fake

import (
	v1 "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (c *FakeRoleBindings) Create(roleBinding *v1.RoleBinding) (result *v1.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(rolebindingsResource, c.ns, roleBinding), &v1.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.RoleBinding), err
}

func (c *FakeRoleBindings) Update(roleBinding *v1.RoleBinding) (result *v1.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(rolebindingsResource, c.ns, roleBinding), &v1.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.RoleBinding), err
}

func (c *FakeRoleBindings) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(rolebindingsResource, c.ns, name), &v1.RoleBinding{})

	return err
}

func (c *FakeRoleBindings) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(rolebindingsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.RoleBindingList{})
	return err
}

func (c *FakeRoleBindings) Get(name string, options meta_v1.GetOptions) (result *v1.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(rolebindingsResource, c.ns, name), &v1.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.RoleBinding), err
}

func (c *FakeRoleBindings) List(opts meta_v1.ListOptions) (result *v1.RoleBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(rolebindingsResource, rolebindingsKind, c.ns, opts), &v1.RoleBindingList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.RoleBindingList{}
	for _, item := range obj.(*v1.RoleBindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested roleBindings.
func (c *FakeRoleBindings) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(rolebindingsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched roleBinding.
func (c *FakeRoleBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.RoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(rolebindingsResource, c.ns, name, data, subresources...), &v1.RoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.RoleBinding), err
}
