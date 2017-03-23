package fake

import (
	v1 "github.com/openshift/origin/pkg/quota/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	api_v1 "k8s.io/kubernetes/pkg/api/v1"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeClusterResourceQuotas implements ClusterResourceQuotaInterface
type FakeClusterResourceQuotas struct {
	Fake *FakeQuotaV1
}

var clusterresourcequotasResource = unversioned.GroupVersionResource{Group: "quota.openshift.io", Version: "v1", Resource: "clusterresourcequotas"}

func (c *FakeClusterResourceQuotas) Create(clusterResourceQuota *v1.ClusterResourceQuota) (result *v1.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootCreateAction(clusterresourcequotasResource, clusterResourceQuota), &v1.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) Update(clusterResourceQuota *v1.ClusterResourceQuota) (result *v1.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootUpdateAction(clusterresourcequotasResource, clusterResourceQuota), &v1.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) UpdateStatus(clusterResourceQuota *v1.ClusterResourceQuota) (*v1.ClusterResourceQuota, error) {
	obj, err := c.Fake.
		Invokes(core.NewRootUpdateSubresourceAction(clusterresourcequotasResource, "status", clusterResourceQuota), &v1.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) Delete(name string, options *api_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewRootDeleteAction(clusterresourcequotasResource, name), &v1.ClusterResourceQuota{})
	return err
}

func (c *FakeClusterResourceQuotas) DeleteCollection(options *api_v1.DeleteOptions, listOptions api_v1.ListOptions) error {
	action := core.NewRootDeleteCollectionAction(clusterresourcequotasResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ClusterResourceQuotaList{})
	return err
}

func (c *FakeClusterResourceQuotas) Get(name string) (result *v1.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootGetAction(clusterresourcequotasResource, name), &v1.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) List(opts api_v1.ListOptions) (result *v1.ClusterResourceQuotaList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootListAction(clusterresourcequotasResource, opts), &v1.ClusterResourceQuotaList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := core.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.ClusterResourceQuotaList{}
	for _, item := range obj.(*v1.ClusterResourceQuotaList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterResourceQuotas.
func (c *FakeClusterResourceQuotas) Watch(opts api_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewRootWatchAction(clusterresourcequotasResource, opts))
}

// Patch applies the patch and returns the patched clusterResourceQuota.
func (c *FakeClusterResourceQuotas) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootPatchSubresourceAction(clusterresourcequotasResource, name, data, subresources...), &v1.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterResourceQuota), err
}
