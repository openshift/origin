package fake

import (
	api "github.com/openshift/origin/pkg/quota/api"
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

func (c *FakeClusterResourceQuotas) Create(clusterResourceQuota *api.ClusterResourceQuota) (result *api.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterresourcequotasResource, clusterResourceQuota), &api.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) Update(clusterResourceQuota *api.ClusterResourceQuota) (result *api.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterresourcequotasResource, clusterResourceQuota), &api.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) UpdateStatus(clusterResourceQuota *api.ClusterResourceQuota) (*api.ClusterResourceQuota, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(clusterresourcequotasResource, "status", clusterResourceQuota), &api.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterresourcequotasResource, name), &api.ClusterResourceQuota{})
	return err
}

func (c *FakeClusterResourceQuotas) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterresourcequotasResource, listOptions)

	_, err := c.Fake.Invokes(action, &api.ClusterResourceQuotaList{})
	return err
}

func (c *FakeClusterResourceQuotas) Get(name string, options v1.GetOptions) (result *api.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterresourcequotasResource, name), &api.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) List(opts v1.ListOptions) (result *api.ClusterResourceQuotaList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterresourcequotasResource, opts), &api.ClusterResourceQuotaList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
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
func (c *FakeClusterResourceQuotas) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterresourcequotasResource, opts))
}

// Patch applies the patch and returns the patched clusterResourceQuota.
func (c *FakeClusterResourceQuotas) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterresourcequotasResource, name, data, subresources...), &api.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterResourceQuota), err
}
