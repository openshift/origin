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

// FakePolicyBindings implements PolicyBindingInterface
type FakePolicyBindings struct {
	Fake *FakeAuthorization
	ns   string
}

var policybindingsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "policybindings"}

var policybindingsKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "", Kind: "PolicyBinding"}

// Get takes name of the policyBinding, and returns the corresponding policyBinding object, and an error if there is any.
func (c *FakePolicyBindings) Get(name string, options v1.GetOptions) (result *authorization.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(policybindingsResource, c.ns, name), &authorization.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.PolicyBinding), err
}

// List takes label and field selectors, and returns the list of PolicyBindings that match those selectors.
func (c *FakePolicyBindings) List(opts v1.ListOptions) (result *authorization.PolicyBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(policybindingsResource, policybindingsKind, c.ns, opts), &authorization.PolicyBindingList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &authorization.PolicyBindingList{}
	for _, item := range obj.(*authorization.PolicyBindingList).Items {
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

// Create takes the representation of a policyBinding and creates it.  Returns the server's representation of the policyBinding, and an error, if there is any.
func (c *FakePolicyBindings) Create(policyBinding *authorization.PolicyBinding) (result *authorization.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(policybindingsResource, c.ns, policyBinding), &authorization.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.PolicyBinding), err
}

// Update takes the representation of a policyBinding and updates it. Returns the server's representation of the policyBinding, and an error, if there is any.
func (c *FakePolicyBindings) Update(policyBinding *authorization.PolicyBinding) (result *authorization.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(policybindingsResource, c.ns, policyBinding), &authorization.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.PolicyBinding), err
}

// Delete takes name of the policyBinding and deletes it. Returns an error if one occurs.
func (c *FakePolicyBindings) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(policybindingsResource, c.ns, name), &authorization.PolicyBinding{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePolicyBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(policybindingsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &authorization.PolicyBindingList{})
	return err
}

// Patch applies the patch and returns the patched policyBinding.
func (c *FakePolicyBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(policybindingsResource, c.ns, name, data, subresources...), &authorization.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.PolicyBinding), err
}
