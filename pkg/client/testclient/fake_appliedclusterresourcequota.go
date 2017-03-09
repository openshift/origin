package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	quotaapi "github.com/openshift/origin/pkg/quota/api"
)

type FakeAppliedClusterResourceQuotas struct {
	Fake      *Fake
	Namespace string
}

var appliedClusterResourceQuotasResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "appliedclusterresourcequotas"}

func (c *FakeAppliedClusterResourceQuotas) Get(name string) (*quotaapi.AppliedClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(appliedClusterResourceQuotasResource, c.Namespace, name), &quotaapi.AppliedClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.AppliedClusterResourceQuota), err
}

func (c *FakeAppliedClusterResourceQuotas) List(opts metainternal.ListOptions) (*quotaapi.AppliedClusterResourceQuotaList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewListAction(appliedClusterResourceQuotasResource, c.Namespace, opts), &quotaapi.AppliedClusterResourceQuotaList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.AppliedClusterResourceQuotaList), err
}
