package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/authorization/api"
)

// FakeClusterResourceQuotas implements ClusterResourceQuotasInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterResourceQuotas struct {
	Fake *Fake
}

func (c *FakeClusterResourceQuotas) Get(name string) (*api.ClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("clusterResourceQuotas", name), &api.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}

	return obj.(*api.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) List(opts kapi.ListOptions) (*api.ClusterResourceQuotaList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("clusterResourceQuotas", opts), &api.ClusterResourceQuotaList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*api.ClusterResourceQuotaList), err
}

func (c *FakeClusterResourceQuotas) Create(inObj *api.ClusterResourceQuota) (*api.ClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("clusterResourceQuotas", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*api.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) Update(inObj *api.ClusterResourceQuota) (*api.ClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootUpdateAction("clusterResourceQuotas", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*api.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("clusterResourceQuotas", name), &api.ClusterResourceQuota{})
	return err
}

func (c *FakeClusterResourceQuotas) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewRootWatchAction("clusterResourceQuotas", opts))
}

func (c *FakeClusterResourceQuotas) UpdateStatus(inObj *api.ClusterResourceQuota) (result *api.ClusterResourceQuota, err error) {
	create := ktestclient.NewRootCreateAction("clusterResourceQuotas", inObj)
	create.Subresource = "status"
	obj, err := c.Fake.Invokes(create, inObj)
	return obj.(*api.ClusterResourceQuota), err
}
