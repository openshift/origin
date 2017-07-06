package fake

import (
	v1 "github.com/openshift/origin/pkg/quota/apis/quota/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterResourceQuotas implements ClusterResourceQuotaInterface
type FakeClusterResourceQuotas struct {
	Fake *FakeQuotaV1
}

var clusterresourcequotasResource = schema.GroupVersionResource{Group: "quota.openshift.io", Version: "v1", Resource: "clusterresourcequotas"}

var clusterresourcequotasKind = schema.GroupVersionKind{Group: "quota.openshift.io", Version: "v1", Kind: "ClusterResourceQuota"}

func (c *FakeClusterResourceQuotas) Create(clusterResourceQuota *v1.ClusterResourceQuota) (result *v1.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterresourcequotasResource, clusterResourceQuota), &v1.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) Update(clusterResourceQuota *v1.ClusterResourceQuota) (result *v1.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterresourcequotasResource, clusterResourceQuota), &v1.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) UpdateStatus(clusterResourceQuota *v1.ClusterResourceQuota) (*v1.ClusterResourceQuota, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(clusterresourcequotasResource, "status", clusterResourceQuota), &v1.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterresourcequotasResource, name), &v1.ClusterResourceQuota{})
	return err
}

func (c *FakeClusterResourceQuotas) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterresourcequotasResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ClusterResourceQuotaList{})
	return err
}

func (c *FakeClusterResourceQuotas) Get(name string, options meta_v1.GetOptions) (result *v1.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterresourcequotasResource, name), &v1.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) List(opts meta_v1.ListOptions) (result *v1.ClusterResourceQuotaList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterresourcequotasResource, clusterresourcequotasKind, opts), &v1.ClusterResourceQuotaList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
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
func (c *FakeClusterResourceQuotas) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterresourcequotasResource, opts))
}

// Patch applies the patch and returns the patched clusterResourceQuota.
func (c *FakeClusterResourceQuotas) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ClusterResourceQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterresourcequotasResource, name, data, subresources...), &v1.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterResourceQuota), err
}
