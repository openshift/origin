package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	sdnapi "github.com/openshift/origin/pkg/sdn/apis/network"
)

// FakeEgressNetworkPolicy implements EgressNetworkPolicyInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeEgressNetworkPolicy struct {
	Fake      *Fake
	Namespace string
}

var egressNetworkPoliciesResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "egressnetworkpolicies"}
var egressNetworkPoliciesKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "EgressNetworkPolicy"}

func (c *FakeEgressNetworkPolicy) Get(name string, options metav1.GetOptions) (*sdnapi.EgressNetworkPolicy, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(egressNetworkPoliciesResource, c.Namespace, name), &sdnapi.EgressNetworkPolicy{})
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.EgressNetworkPolicy), err
}

func (c *FakeEgressNetworkPolicy) List(opts metav1.ListOptions) (*sdnapi.EgressNetworkPolicyList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewListAction(egressNetworkPoliciesResource, egressNetworkPoliciesKind, c.Namespace, opts), &sdnapi.EgressNetworkPolicyList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.EgressNetworkPolicyList), err
}

func (c *FakeEgressNetworkPolicy) Create(inObj *sdnapi.EgressNetworkPolicy) (*sdnapi.EgressNetworkPolicy, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(egressNetworkPoliciesResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.EgressNetworkPolicy), err
}

func (c *FakeEgressNetworkPolicy) Update(inObj *sdnapi.EgressNetworkPolicy) (*sdnapi.EgressNetworkPolicy, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewUpdateAction(egressNetworkPoliciesResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.EgressNetworkPolicy), err
}

func (c *FakeEgressNetworkPolicy) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewDeleteAction(egressNetworkPoliciesResource, c.Namespace, name), &sdnapi.EgressNetworkPolicy{})
	return err
}

func (c *FakeEgressNetworkPolicy) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewWatchAction(egressNetworkPoliciesResource, c.Namespace, opts))
}
