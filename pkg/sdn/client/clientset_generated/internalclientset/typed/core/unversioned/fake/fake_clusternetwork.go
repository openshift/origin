package fake

import (
	api "github.com/openshift/origin/pkg/sdn/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
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

var clusternetworksResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "clusternetworks"}

func (c *FakeClusterNetworks) Create(clusterNetwork *api.ClusterNetwork) (result *api.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(clusternetworksResource, c.ns, clusterNetwork), &api.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterNetwork), err
}

func (c *FakeClusterNetworks) Update(clusterNetwork *api.ClusterNetwork) (result *api.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(clusternetworksResource, c.ns, clusterNetwork), &api.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterNetwork), err
}

func (c *FakeClusterNetworks) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(clusternetworksResource, c.ns, name), &api.ClusterNetwork{})

	return err
}

func (c *FakeClusterNetworks) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewDeleteCollectionAction(clusternetworksResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.ClusterNetworkList{})
	return err
}

func (c *FakeClusterNetworks) Get(name string) (result *api.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(clusternetworksResource, c.ns, name), &api.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterNetwork), err
}

func (c *FakeClusterNetworks) List(opts pkg_api.ListOptions) (result *api.ClusterNetworkList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(clusternetworksResource, c.ns, opts), &api.ClusterNetworkList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
	if label == nil {
		label = labels.Everything()
	}
	list := &api.ClusterNetworkList{}
	for _, item := range obj.(*api.ClusterNetworkList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterNetworks.
func (c *FakeClusterNetworks) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(clusternetworksResource, c.ns, opts))

}

// Patch applies the patch and returns the patched clusterNetwork.
func (c *FakeClusterNetworks) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ClusterNetwork, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(clusternetworksResource, c.ns, name, data, subresources...), &api.ClusterNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterNetwork), err
}
