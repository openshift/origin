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

// FakePolicies implements PolicyInterface
type FakePolicies struct {
	Fake *FakeAuthorizationV1
	ns   string
}

var policiesResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "v1", Resource: "policies"}

var policiesKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "v1", Kind: "Policy"}

// Get takes name of the policy, and returns the corresponding policy object, and an error if there is any.
func (c *FakePolicies) Get(name string, options v1.GetOptions) (result *authorization_v1.Policy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(policiesResource, c.ns, name), &authorization_v1.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.Policy), err
}

// List takes label and field selectors, and returns the list of Policies that match those selectors.
func (c *FakePolicies) List(opts v1.ListOptions) (result *authorization_v1.PolicyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(policiesResource, policiesKind, c.ns, opts), &authorization_v1.PolicyList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &authorization_v1.PolicyList{}
	for _, item := range obj.(*authorization_v1.PolicyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested policies.
func (c *FakePolicies) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(policiesResource, c.ns, opts))

}

// Create takes the representation of a policy and creates it.  Returns the server's representation of the policy, and an error, if there is any.
func (c *FakePolicies) Create(policy *authorization_v1.Policy) (result *authorization_v1.Policy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(policiesResource, c.ns, policy), &authorization_v1.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.Policy), err
}

// Update takes the representation of a policy and updates it. Returns the server's representation of the policy, and an error, if there is any.
func (c *FakePolicies) Update(policy *authorization_v1.Policy) (result *authorization_v1.Policy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(policiesResource, c.ns, policy), &authorization_v1.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.Policy), err
}

// Delete takes name of the policy and deletes it. Returns an error if one occurs.
func (c *FakePolicies) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(policiesResource, c.ns, name), &authorization_v1.Policy{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePolicies) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(policiesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &authorization_v1.PolicyList{})
	return err
}

// Patch applies the patch and returns the patched policy.
func (c *FakePolicies) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization_v1.Policy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(policiesResource, c.ns, name, data, subresources...), &authorization_v1.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization_v1.Policy), err
}
