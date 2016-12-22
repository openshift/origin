package fake

import (
	v1 "github.com/openshift/origin/pkg/sdn/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeClusterNetworks implements ClusterNetworkInterface
type FakeClusterNetworks struct {
	Fake *FakeCore
	ns   string
}

var clusternetworksResource = unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "clusternetworks"}

func (c *FakeClusterNetworks) Create(clusterNetwork *v1.ClusterNetwork) (result *v1.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(clusternetworksResource, c.ns, clusterNetwork), &v1.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterNetwork), err
}

func (c *FakeClusterNetworks) Update(clusterNetwork *v1.ClusterNetwork) (result *v1.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(clusternetworksResource, c.ns, clusterNetwork), &v1.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterNetwork), err
}

func (c *FakeClusterNetworks) Delete(name string, options *api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(clusternetworksResource, c.ns, name), &v1.ClusterNetwork{})

	return err
}

func (c *FakeClusterNetworks) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	action := core.NewDeleteCollectionAction(clusternetworksResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ClusterNetworkList{})
	return err
}

func (c *FakeClusterNetworks) Get(name string) (result *v1.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(clusternetworksResource, c.ns, name), &v1.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterNetwork), err
}

func (c *FakeClusterNetworks) List(opts api.ListOptions) (result *v1.ClusterNetworkList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(clusternetworksResource, c.ns, opts), &v1.ClusterNetworkList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
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
func (c *FakeClusterNetworks) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(clusternetworksResource, c.ns, opts))

}

// Patch applies the patch and returns the patched clusterNetwork.
func (c *FakeClusterNetworks) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(clusternetworksResource, c.ns, name, data, subresources...), &v1.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterNetwork), err
}
