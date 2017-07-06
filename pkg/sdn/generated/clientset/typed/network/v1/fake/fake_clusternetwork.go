package fake

import (
	v1 "github.com/openshift/origin/pkg/sdn/apis/network/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterNetworks implements ClusterNetworkInterface
type FakeClusterNetworks struct {
	Fake *FakeNetworkV1
	ns   string
}

var clusternetworksResource = schema.GroupVersionResource{Group: "network.openshift.io", Version: "v1", Resource: "clusternetworks"}

var clusternetworksKind = schema.GroupVersionKind{Group: "network.openshift.io", Version: "v1", Kind: "ClusterNetwork"}

func (c *FakeClusterNetworks) Create(clusterNetwork *v1.ClusterNetwork) (result *v1.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(clusternetworksResource, c.ns, clusterNetwork), &v1.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterNetwork), err
}

func (c *FakeClusterNetworks) Update(clusterNetwork *v1.ClusterNetwork) (result *v1.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(clusternetworksResource, c.ns, clusterNetwork), &v1.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterNetwork), err
}

func (c *FakeClusterNetworks) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(clusternetworksResource, c.ns, name), &v1.ClusterNetwork{})

	return err
}

func (c *FakeClusterNetworks) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(clusternetworksResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ClusterNetworkList{})
	return err
}

func (c *FakeClusterNetworks) Get(name string, options meta_v1.GetOptions) (result *v1.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(clusternetworksResource, c.ns, name), &v1.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterNetwork), err
}

func (c *FakeClusterNetworks) List(opts meta_v1.ListOptions) (result *v1.ClusterNetworkList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(clusternetworksResource, clusternetworksKind, c.ns, opts), &v1.ClusterNetworkList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.ClusterNetworkList{}
	for _, item := range obj.(*v1.ClusterNetworkList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterNetworks.
func (c *FakeClusterNetworks) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(clusternetworksResource, c.ns, opts))

}

// Patch applies the patch and returns the patched clusterNetwork.
func (c *FakeClusterNetworks) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(clusternetworksResource, c.ns, name, data, subresources...), &v1.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterNetwork), err
}
