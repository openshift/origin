package fake

import (
	quota "github.com/openshift/origin/pkg/quota/apis/quota"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterResourceQuotas implements ClusterResourceQuotaInterface
type FakeClusterResourceQuotas struct {
	Fake *FakeQuota
}

var clusterresourcequotasResource = schema.GroupVersionResource{Group: "quota.openshift.io", Version: "", Resource: "clusterresourcequotas"}

var clusterresourcequotasKind = schema.GroupVersionKind{Group: "quota.openshift.io", Version: "", Kind: "ClusterResourceQuota"}

func (c *FakeClusterResourceQuotas) Create(clusterResourceQuota *quota.ClusterResourceQuota) (result *quota.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterresourcequotasResource, clusterResourceQuota), &quota.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*quota.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) Update(clusterResourceQuota *quota.ClusterResourceQuota) (result *quota.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterresourcequotasResource, clusterResourceQuota), &quota.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*quota.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) UpdateStatus(clusterResourceQuota *quota.ClusterResourceQuota) (*quota.ClusterResourceQuota, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(clusterresourcequotasResource, "status", clusterResourceQuota), &quota.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*quota.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterresourcequotasResource, name), &quota.ClusterResourceQuota{})
	return err
}

func (c *FakeClusterResourceQuotas) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterresourcequotasResource, listOptions)

	_, err := c.Fake.Invokes(action, &quota.ClusterResourceQuotaList{})
	return err
}

func (c *FakeClusterResourceQuotas) Get(name string, options v1.GetOptions) (result *quota.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterresourcequotasResource, name), &quota.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*quota.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) List(opts v1.ListOptions) (result *quota.ClusterResourceQuotaList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterresourcequotasResource, clusterresourcequotasKind, opts), &quota.ClusterResourceQuotaList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &quota.ClusterResourceQuotaList{}
	for _, item := range obj.(*quota.ClusterResourceQuotaList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterResourceQuotas.
func (c *FakeClusterResourceQuotas) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterresourcequotasResource, opts))
}

// Patch applies the patch and returns the patched clusterResourceQuota.
func (c *FakeClusterResourceQuotas) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *quota.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterresourcequotasResource, name, data, subresources...), &quota.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*quota.ClusterResourceQuota), err
}
