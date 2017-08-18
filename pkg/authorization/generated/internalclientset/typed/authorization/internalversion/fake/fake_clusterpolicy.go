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

// FakeClusterPolicies implements ClusterPolicyInterface
type FakeClusterPolicies struct {
	Fake *FakeAuthorization
}

var clusterpoliciesResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "clusterpolicies"}

var clusterpoliciesKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "", Kind: "ClusterPolicy"}

// Get takes name of the clusterPolicy, and returns the corresponding clusterPolicy object, and an error if there is any.
func (c *FakeClusterPolicies) Get(name string, options v1.GetOptions) (result *authorization.ClusterPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterpoliciesResource, name), &authorization.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.ClusterPolicy), err
}

// List takes label and field selectors, and returns the list of ClusterPolicies that match those selectors.
func (c *FakeClusterPolicies) List(opts v1.ListOptions) (result *authorization.ClusterPolicyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterpoliciesResource, clusterpoliciesKind, opts), &authorization.ClusterPolicyList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &authorization.ClusterPolicyList{}
	for _, item := range obj.(*authorization.ClusterPolicyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterPolicies.
func (c *FakeClusterPolicies) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterpoliciesResource, opts))
}

// Create takes the representation of a clusterPolicy and creates it.  Returns the server's representation of the clusterPolicy, and an error, if there is any.
func (c *FakeClusterPolicies) Create(clusterPolicy *authorization.ClusterPolicy) (result *authorization.ClusterPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterpoliciesResource, clusterPolicy), &authorization.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.ClusterPolicy), err
}

// Update takes the representation of a clusterPolicy and updates it. Returns the server's representation of the clusterPolicy, and an error, if there is any.
func (c *FakeClusterPolicies) Update(clusterPolicy *authorization.ClusterPolicy) (result *authorization.ClusterPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterpoliciesResource, clusterPolicy), &authorization.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.ClusterPolicy), err
}

// Delete takes name of the clusterPolicy and deletes it. Returns an error if one occurs.
func (c *FakeClusterPolicies) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterpoliciesResource, name), &authorization.ClusterPolicy{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterPolicies) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterpoliciesResource, listOptions)

	_, err := c.Fake.Invokes(action, &authorization.ClusterPolicyList{})
	return err
}

// Patch applies the patch and returns the patched clusterPolicy.
func (c *FakeClusterPolicies) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization.ClusterPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterpoliciesResource, name, data, subresources...), &authorization.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.ClusterPolicy), err
}
