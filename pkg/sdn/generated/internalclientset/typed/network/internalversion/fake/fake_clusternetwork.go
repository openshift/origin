package fake

import (
	network "github.com/openshift/origin/pkg/sdn/apis/network"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterNetworks implements ClusterNetworkInterface
type FakeClusterNetworks struct {
	Fake *FakeNetwork
	ns   string
}

var clusternetworksResource = schema.GroupVersionResource{Group: "network.openshift.io", Version: "", Resource: "clusternetworks"}

var clusternetworksKind = schema.GroupVersionKind{Group: "network.openshift.io", Version: "", Kind: "ClusterNetwork"}

func (c *FakeClusterNetworks) Create(clusterNetwork *network.ClusterNetwork) (result *network.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(clusternetworksResource, c.ns, clusterNetwork), &network.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*network.ClusterNetwork), err
}

func (c *FakeClusterNetworks) Update(clusterNetwork *network.ClusterNetwork) (result *network.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(clusternetworksResource, c.ns, clusterNetwork), &network.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*network.ClusterNetwork), err
}

func (c *FakeClusterNetworks) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(clusternetworksResource, c.ns, name), &network.ClusterNetwork{})

	return err
}

func (c *FakeClusterNetworks) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(clusternetworksResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &network.ClusterNetworkList{})
	return err
}

func (c *FakeClusterNetworks) Get(name string, options v1.GetOptions) (result *network.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(clusternetworksResource, c.ns, name), &network.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*network.ClusterNetwork), err
}

func (c *FakeClusterNetworks) List(opts v1.ListOptions) (result *network.ClusterNetworkList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(clusternetworksResource, clusternetworksKind, c.ns, opts), &network.ClusterNetworkList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &network.ClusterNetworkList{}
	for _, item := range obj.(*network.ClusterNetworkList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterNetworks.
func (c *FakeClusterNetworks) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(clusternetworksResource, c.ns, opts))

}

// Patch applies the patch and returns the patched clusterNetwork.
func (c *FakeClusterNetworks) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *network.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(clusternetworksResource, c.ns, name, data, subresources...), &network.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*network.ClusterNetwork), err
}
