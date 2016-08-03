package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/watch"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

// FakeEgressNetworkPolicy implements EgressNetworkPolicyInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeEgressNetworkPolicy struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeEgressNetworkPolicy) Get(name string) (*sdnapi.EgressNetworkPolicy, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewGetAction("egressnetworkpolicies", c.Namespace, name), &sdnapi.EgressNetworkPolicy{})
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.EgressNetworkPolicy), err
}

func (c *FakeEgressNetworkPolicy) List(opts kapi.ListOptions) (*sdnapi.EgressNetworkPolicyList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewListAction("egressnetworkpolicies", c.Namespace, opts), &sdnapi.EgressNetworkPolicyList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.EgressNetworkPolicyList), err
}

func (c *FakeEgressNetworkPolicy) Create(inObj *sdnapi.EgressNetworkPolicy) (*sdnapi.EgressNetworkPolicy, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewCreateAction("egressnetworkpolicies", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.EgressNetworkPolicy), err
}

func (c *FakeEgressNetworkPolicy) Update(inObj *sdnapi.EgressNetworkPolicy) (*sdnapi.EgressNetworkPolicy, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewUpdateAction("egressnetworkpolicies", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*sdnapi.EgressNetworkPolicy), err
}

func (c *FakeEgressNetworkPolicy) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewDeleteAction("egressnetworkpolicies", c.Namespace, name), &sdnapi.EgressNetworkPolicy{})
	return err
}

func (c *FakeEgressNetworkPolicy) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewWatchAction("egressnetworkpolicies", c.Namespace, opts))
}
