package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	core "k8s.io/client-go/testing"

	quotaapi "github.com/openshift/origin/pkg/quota/api"
)

type FakeAppliedClusterResourceQuotas struct {
	Fake      *Fake
	Namespace string
}

var appliedClusterResourceQuotasResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "appliedclusterresourcequotas"}

func (c *FakeAppliedClusterResourceQuotas) Get(name string, options metav1.GetOptions) (*quotaapi.AppliedClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(core.NewGetAction(appliedClusterResourceQuotasResource, c.Namespace, name), &quotaapi.AppliedClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.AppliedClusterResourceQuota), err
}

func (c *FakeAppliedClusterResourceQuotas) List(opts metainternal.ListOptions) (*quotaapi.AppliedClusterResourceQuotaList, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	obj, err := c.Fake.Invokes(core.NewListAction(appliedClusterResourceQuotasResource, c.Namespace, optsv1), &quotaapi.AppliedClusterResourceQuotaList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.AppliedClusterResourceQuotaList), err
}
