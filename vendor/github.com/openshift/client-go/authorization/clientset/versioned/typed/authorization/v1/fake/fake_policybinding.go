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

// FakePolicyBindings implements PolicyBindingInterface
type FakePolicyBindings struct {
	Fake *FakeAuthorizationV1
	ns   string
}

var policybindingsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "v1", Resource: "policybindings"}

var policybindingsKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "v1", Kind: "PolicyBinding"}

// Get takes name of the policyBinding, and returns the corresponding policyBinding object, and an error if there is any.
func (c *FakePolicyBindings) Get(name string, options v1.GetOptions) (result *authorization_v1.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(policybindingsResource, c.ns, name), &authorization_v1.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.PolicyBinding), err
}

// List takes label and field selectors, and returns the list of PolicyBindings that match those selectors.
func (c *FakePolicyBindings) List(opts v1.ListOptions) (result *authorization_v1.PolicyBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(policybindingsResource, policybindingsKind, c.ns, opts), &authorization_v1.PolicyBindingList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &authorization_v1.PolicyBindingList{}
	for _, item := range obj.(*authorization_v1.PolicyBindingList).Items {
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
func (c *FakePolicyBindings) Create(policyBinding *authorization_v1.PolicyBinding) (result *authorization_v1.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(policybindingsResource, c.ns, policyBinding), &authorization_v1.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.PolicyBinding), err
}

// Update takes the representation of a policyBinding and updates it. Returns the server's representation of the policyBinding, and an error, if there is any.
func (c *FakePolicyBindings) Update(policyBinding *authorization_v1.PolicyBinding) (result *authorization_v1.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(policybindingsResource, c.ns, policyBinding), &authorization_v1.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.PolicyBinding), err
}

// Delete takes name of the policyBinding and deletes it. Returns an error if one occurs.
func (c *FakePolicyBindings) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(policybindingsResource, c.ns, name), &authorization_v1.PolicyBinding{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePolicyBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(policybindingsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &authorization_v1.PolicyBindingList{})
	return err
}

// Patch applies the patch and returns the patched policyBinding.
func (c *FakePolicyBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization_v1.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(policybindingsResource, c.ns, name, data, subresources...), &authorization_v1.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.PolicyBinding), err
}
