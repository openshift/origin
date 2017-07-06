package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
)

type FakeAppliedClusterResourceQuotas struct {
	Fake      *Fake
	Namespace string
}

var appliedClusterResourceQuotasResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "appliedclusterresourcequotas"}
var appliedClusterResourceQuotasKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "AppliedClusterResourceQuota"}

func (c *FakeAppliedClusterResourceQuotas) Get(name string, options metav1.GetOptions) (*quotaapi.AppliedClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(appliedClusterResourceQuotasResource, c.Namespace, name), &quotaapi.AppliedClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.AppliedClusterResourceQuota), err
}

func (c *FakeAppliedClusterResourceQuotas) List(opts metav1.ListOptions) (*quotaapi.AppliedClusterResourceQuotaList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewListAction(appliedClusterResourceQuotasResource, appliedClusterResourceQuotasKind, c.Namespace, opts), &quotaapi.AppliedClusterResourceQuotaList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.AppliedClusterResourceQuotaList), err
}
