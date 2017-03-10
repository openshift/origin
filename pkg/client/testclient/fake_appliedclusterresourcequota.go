package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/client/testing/core"

	quotaapi "github.com/openshift/origin/pkg/quota/api"
)

type FakeAppliedClusterResourceQuotas struct {
	Fake      *Fake
	Namespace string
}

var appliedClusterResourceQuotasResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "appliedclusterresourcequotas"}

func (c *FakeAppliedClusterResourceQuotas) Get(name string) (*quotaapi.AppliedClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(core.NewGetAction(appliedClusterResourceQuotasResource, c.Namespace, name), &quotaapi.AppliedClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.AppliedClusterResourceQuota), err
}

func (c *FakeAppliedClusterResourceQuotas) List(opts metainternal.ListOptions) (*quotaapi.AppliedClusterResourceQuotaList, error) {
	obj, err := c.Fake.Invokes(core.NewListAction(appliedClusterResourceQuotasResource, c.Namespace, opts), &quotaapi.AppliedClusterResourceQuotaList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.AppliedClusterResourceQuotaList), err
}
