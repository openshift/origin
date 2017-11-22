package fake

import (
	network_v1 "github.com/openshift/api/network/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeEgressNetworkPolicies implements EgressNetworkPolicyInterface
type FakeEgressNetworkPolicies struct {
	Fake *FakeNetworkV1
	ns   string
}

var egressnetworkpoliciesResource = schema.GroupVersionResource{Group: "network.openshift.io", Version: "v1", Resource: "egressnetworkpolicies"}

var egressnetworkpoliciesKind = schema.GroupVersionKind{Group: "network.openshift.io", Version: "v1", Kind: "EgressNetworkPolicy"}

// Get takes name of the egressNetworkPolicy, and returns the corresponding egressNetworkPolicy object, and an error if there is any.
func (c *FakeEgressNetworkPolicies) Get(name string, options v1.GetOptions) (result *network_v1.EgressNetworkPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(egressnetworkpoliciesResource, c.ns, name), &network_v1.EgressNetworkPolicy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*network_v1.EgressNetworkPolicy), err
}

// List takes label and field selectors, and returns the list of EgressNetworkPolicies that match those selectors.
func (c *FakeEgressNetworkPolicies) List(opts v1.ListOptions) (result *network_v1.EgressNetworkPolicyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(egressnetworkpoliciesResource, egressnetworkpoliciesKind, c.ns, opts), &network_v1.EgressNetworkPolicyList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &network_v1.EgressNetworkPolicyList{}
	for _, item := range obj.(*network_v1.EgressNetworkPolicyList).Items {
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
func (c *FakeEgressNetworkPolicies) Create(egressNetworkPolicy *network_v1.EgressNetworkPolicy) (result *network_v1.EgressNetworkPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(egressnetworkpoliciesResource, c.ns, egressNetworkPolicy), &network_v1.EgressNetworkPolicy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*network_v1.EgressNetworkPolicy), err
}

// Update takes the representation of a egressNetworkPolicy and updates it. Returns the server's representation of the egressNetworkPolicy, and an error, if there is any.
func (c *FakeEgressNetworkPolicies) Update(egressNetworkPolicy *network_v1.EgressNetworkPolicy) (result *network_v1.EgressNetworkPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(egressnetworkpoliciesResource, c.ns, egressNetworkPolicy), &network_v1.EgressNetworkPolicy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*network_v1.EgressNetworkPolicy), err
}

// Delete takes name of the egressNetworkPolicy and deletes it. Returns an error if one occurs.
func (c *FakeEgressNetworkPolicies) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(egressnetworkpoliciesResource, c.ns, name), &network_v1.EgressNetworkPolicy{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeEgressNetworkPolicies) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(egressnetworkpoliciesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &network_v1.EgressNetworkPolicyList{})
	return err
}

// Patch applies the patch and returns the patched egressNetworkPolicy.
func (c *FakeEgressNetworkPolicies) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *network_v1.EgressNetworkPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(egressnetworkpoliciesResource, c.ns, name, data, subresources...), &network_v1.EgressNetworkPolicy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*network_v1.EgressNetworkPolicy), err
}
