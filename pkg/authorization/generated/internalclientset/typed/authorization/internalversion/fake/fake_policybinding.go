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

// FakePolicyBindings implements PolicyBindingInterface
type FakePolicyBindings struct {
	Fake *FakeAuthorization
	ns   string
}

var policybindingsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "policybindings"}

func (c *FakePolicyBindings) Create(policyBinding *api.PolicyBinding) (result *api.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(policybindingsResource, c.ns, policyBinding), &api.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.PolicyBinding), err
}

func (c *FakePolicyBindings) Update(policyBinding *api.PolicyBinding) (result *api.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(policybindingsResource, c.ns, policyBinding), &api.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.PolicyBinding), err
}

func (c *FakePolicyBindings) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(policybindingsResource, c.ns, name), &api.PolicyBinding{})

	return err
}

func (c *FakePolicyBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(policybindingsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.PolicyBindingList{})
	return err
}

func (c *FakePolicyBindings) Get(name string, options v1.GetOptions) (result *api.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(policybindingsResource, c.ns, name), &api.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.PolicyBinding), err
}

func (c *FakePolicyBindings) List(opts v1.ListOptions) (result *api.PolicyBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(policybindingsResource, c.ns, opts), &api.PolicyBindingList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.PolicyBindingList{}
	for _, item := range obj.(*api.PolicyBindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested policyBindings.
func (c *FakePolicyBindings) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(policybindingsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched policyBinding.
func (c *FakePolicyBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(policybindingsResource, c.ns, name, data, subresources...), &api.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.PolicyBinding), err
}
