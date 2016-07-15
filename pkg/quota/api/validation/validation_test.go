package validation

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util/validation/field"
	"testing"

	quotaapi "github.com/openshift/origin/pkg/quota/api"
)

func TestValidationClusterQuota(t *testing.T) {
	spec := api.ResourceQuotaSpec{
		Hard: api.ResourceList{
			api.ResourceCPU:                    resource.MustParse("100"),
			api.ResourceMemory:                 resource.MustParse("10000"),
			api.ResourceRequestsCPU:            resource.MustParse("100"),
			api.ResourceRequestsMemory:         resource.MustParse("10000"),
			api.ResourceLimitsCPU:              resource.MustParse("100"),
			api.ResourceLimitsMemory:           resource.MustParse("10000"),
			api.ResourcePods:                   resource.MustParse("10"),
			api.ResourceServices:               resource.MustParse("0"),
			api.ResourceReplicationControllers: resource.MustParse("10"),
			api.ResourceQuotas:                 resource.MustParse("10"),
			api.ResourceConfigMaps:             resource.MustParse("10"),
			api.ResourceSecrets:                resource.MustParse("10"),
		},
	}

	// storage is not yet supported as a quota tracked resource
	invalidQuotaResourceSpec := api.ResourceQuotaSpec{
		Hard: api.ResourceList{
			api.ResourceStorage: resource.MustParse("10"),
		},
	}
	validLabels := map[string]string{"a": "b"}

	errs := ValidateClusterResourceQuota(
		&quotaapi.ClusterResourceQuota{
			ObjectMeta: api.ObjectMeta{Name: "good"},
			Spec: quotaapi.ClusterResourceQuotaSpec{
				Selector: quotaapi.ClusterResourceQuotaSelector{LabelSelector: &unversioned.LabelSelector{MatchLabels: validLabels}},
				Quota:    spec,
			},
		},
	)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A quotaapi.ClusterResourceQuota
		T field.ErrorType
		F string
	}{
		"non-zero-length namespace": {
			A: quotaapi.ClusterResourceQuota{
				ObjectMeta: api.ObjectMeta{Namespace: "bad", Name: "good"},
				Spec: quotaapi.ClusterResourceQuotaSpec{
					Selector: quotaapi.ClusterResourceQuotaSelector{LabelSelector: &unversioned.LabelSelector{MatchLabels: validLabels}},
					Quota:    spec,
				},
			},
			T: field.ErrorTypeForbidden,
			F: "metadata.namespace",
		},
		"missing label selector": {
			A: quotaapi.ClusterResourceQuota{
				ObjectMeta: api.ObjectMeta{Name: "good"},
				Spec: quotaapi.ClusterResourceQuotaSpec{
					Quota: spec,
				},
			},
			T: field.ErrorTypeRequired,
			F: "spec.selector",
		},
		"bad quota spec": {
			A: quotaapi.ClusterResourceQuota{
				ObjectMeta: api.ObjectMeta{Name: "good"},
				Spec: quotaapi.ClusterResourceQuotaSpec{
					Selector: quotaapi.ClusterResourceQuotaSelector{LabelSelector: &unversioned.LabelSelector{MatchLabels: validLabels}},
					Quota:    invalidQuotaResourceSpec,
				},
			},
			T: field.ErrorTypeInvalid,
			F: "spec.quota.hard[storage]",
		},
	}
	for k, v := range errorCases {
		errs := ValidateClusterResourceQuota(&v.A)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}
