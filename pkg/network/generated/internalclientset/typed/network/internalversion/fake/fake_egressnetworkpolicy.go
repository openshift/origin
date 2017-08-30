package fake

import (
	network "github.com/openshift/origin/pkg/network/apis/network"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeEgressNetworkPolicies implements EgressNetworkPolicyInterface
type FakeEgressNetworkPolicies struct {
	Fake *FakeNetwork
	ns   string
}

var egressnetworkpoliciesResource = schema.GroupVersionResource{Group: "network.openshift.io", Version: "", Resource: "egressnetworkpolicies"}

var egressnetworkpoliciesKind = schema.GroupVersionKind{Group: "network.openshift.io", Version: "", Kind: "EgressNetworkPolicy"}

// Get takes name of the egressNetworkPolicy, and returns the corresponding egressNetworkPolicy object, and an error if there is any.
func (c *FakeEgressNetworkPolicies) Get(name string, options v1.GetOptions) (result *network.EgressNetworkPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(egressnetworkpoliciesResource, c.ns, name), &network.EgressNetworkPolicy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*network.EgressNetworkPolicy), err
}

// List takes label and field selectors, and returns the list of EgressNetworkPolicies that match those selectors.
func (c *FakeEgressNetworkPolicies) List(opts v1.ListOptions) (result *network.EgressNetworkPolicyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(egressnetworkpoliciesResource, egressnetworkpoliciesKind, c.ns, opts), &network.EgressNetworkPolicyList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &network.EgressNetworkPolicyList{}
	for _, item := range obj.(*network.EgressNetworkPolicyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested egressNetworkPolicies.
func (c *FakeEgressNetworkPolicies) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(egressnetworkpoliciesResource, c.ns, opts))

}

// Create takes the representation of a egressNetworkPolicy and creates it.  Returns the server's representation of the egressNetworkPolicy, and an error, if there is any.
func (c *FakeEgressNetworkPolicies) Create(egressNetworkPolicy *network.EgressNetworkPolicy) (result *network.EgressNetworkPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(egressnetworkpoliciesResource, c.ns, egressNetworkPolicy), &network.EgressNetworkPolicy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*network.EgressNetworkPolicy), err
}

// Update takes the representation of a egressNetworkPolicy and updates it. Returns the server's representation of the egressNetworkPolicy, and an error, if there is any.
func (c *FakeEgressNetworkPolicies) Update(egressNetworkPolicy *network.EgressNetworkPolicy) (result *network.EgressNetworkPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(egressnetworkpoliciesResource, c.ns, egressNetworkPolicy), &network.EgressNetworkPolicy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*network.EgressNetworkPolicy), err
}

// Delete takes name of the egressNetworkPolicy and deletes it. Returns an error if one occurs.
func (c *FakeEgressNetworkPolicies) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(egressnetworkpoliciesResource, c.ns, name), &network.EgressNetworkPolicy{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeEgressNetworkPolicies) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(egressnetworkpoliciesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &network.EgressNetworkPolicyList{})
	return err
}

// Patch applies the patch and returns the patched egressNetworkPolicy.
func (c *FakeEgressNetworkPolicies) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *network.EgressNetworkPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(egressnetworkpoliciesResource, c.ns, name, data, subresources...), &network.EgressNetworkPolicy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*network.EgressNetworkPolicy), err
}
