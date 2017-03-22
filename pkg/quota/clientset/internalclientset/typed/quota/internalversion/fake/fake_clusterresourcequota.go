package fake

import (
	api "github.com/openshift/origin/pkg/quota/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeClusterResourceQuotas implements ClusterResourceQuotaInterface
type FakeClusterResourceQuotas struct {
	Fake *FakeQuota
}

var clusterresourcequotasResource = unversioned.GroupVersionResource{Group: "quota.openshift.io", Version: "", Resource: "clusterresourcequotas"}

func (c *FakeClusterResourceQuotas) Create(clusterResourceQuota *api.ClusterResourceQuota) (result *api.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootCreateAction(clusterresourcequotasResource, clusterResourceQuota), &api.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) Update(clusterResourceQuota *api.ClusterResourceQuota) (result *api.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootUpdateAction(clusterresourcequotasResource, clusterResourceQuota), &api.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewRootDeleteAction(clusterresourcequotasResource, name), &api.ClusterResourceQuota{})
	return err
}

func (c *FakeClusterResourceQuotas) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewRootDeleteCollectionAction(clusterresourcequotasResource, listOptions)

	_, err := c.Fake.Invokes(action, &api.ClusterResourceQuotaList{})
	return err
}

func (c *FakeClusterResourceQuotas) Get(name string) (result *api.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootGetAction(clusterresourcequotasResource, name), &api.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) List(opts pkg_api.ListOptions) (result *api.ClusterResourceQuotaList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootListAction(clusterresourcequotasResource, opts), &api.ClusterResourceQuotaList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := core.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.ClusterResourceQuotaList{}
	for _, item := range obj.(*api.ClusterResourceQuotaList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterResourceQuotas.
func (c *FakeClusterResourceQuotas) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewRootWatchAction(clusterresourcequotasResource, opts))
}

// Patch applies the patch and returns the patched clusterResourceQuota.
func (c *FakeClusterResourceQuotas) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootPatchSubresourceAction(clusterresourcequotasResource, name, data, subresources...), &api.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterResourceQuota), err
}
