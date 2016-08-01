package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/watch"

	quotaapi "github.com/openshift/origin/pkg/quota/api"
)

// FakeClusterResourceQuotas implements ClusterResourceQuotaInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterResourceQuotas struct {
	Fake *Fake
}

func (c *FakeClusterResourceQuotas) Get(name string) (*quotaapi.ClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("clusterresourcequotas", name), &quotaapi.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) List(opts kapi.ListOptions) (*quotaapi.ClusterResourceQuotaList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("clusterresourcequotas", opts), &quotaapi.ClusterResourceQuotaList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.ClusterResourceQuotaList), err
}

func (c *FakeClusterResourceQuotas) Create(inObj *quotaapi.ClusterResourceQuota) (*quotaapi.ClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("clusterresourcequotas", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) Update(inObj *quotaapi.ClusterResourceQuota) (*quotaapi.ClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootUpdateAction("clusterresourcequotas", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.ClusterResourceQuota), err
}
func (c *FakeClusterResourceQuotas) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("clusterresourcequotas", name), &quotaapi.ClusterResourceQuota{})
	return err
}

func (c *FakeClusterResourceQuotas) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewRootWatchAction("clusterresourcequotas", opts))
}

func (c *FakeClusterResourceQuotas) UpdateStatus(inObj *quotaapi.ClusterResourceQuota) (*quotaapi.ClusterResourceQuota, error) {
	action := ktestclient.UpdateActionImpl{}
	action.Verb = "update"
	action.Resource = "clusterresourcequotas"
	action.Subresource = "status"
	action.Object = inObj

	obj, err := c.Fake.Invokes(action, inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.ClusterResourceQuota), err

}
