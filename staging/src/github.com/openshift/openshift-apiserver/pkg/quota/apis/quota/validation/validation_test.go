package validation

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/core/validation"

	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
)

func spec(scopes ...api.ResourceQuotaScope) api.ResourceQuotaSpec {
	return api.ResourceQuotaSpec{
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
		Scopes: scopes,
	}
}

func scopeableSpec(scopes ...api.ResourceQuotaScope) api.ResourceQuotaSpec {
	return api.ResourceQuotaSpec{
		Hard: api.ResourceList{
			api.ResourceCPU:            resource.MustParse("100"),
			api.ResourceMemory:         resource.MustParse("10000"),
			api.ResourceRequestsCPU:    resource.MustParse("100"),
			api.ResourceRequestsMemory: resource.MustParse("10000"),
			api.ResourceLimitsCPU:      resource.MustParse("100"),
			api.ResourceLimitsMemory:   resource.MustParse("10000"),
		},
		Scopes: scopes,
	}
}

func TestValidationClusterQuota(t *testing.T) {
	// storage is not yet supported as a quota tracked resource
	invalidQuotaResourceSpec := api.ResourceQuotaSpec{
		Hard: api.ResourceList{
			api.ResourceStorage: resource.MustParse("10"),
		},
	}
	validLabels := map[string]string{"a": "b"}

	errs := ValidateClusterResourceQuota(
		&quotaapi.ClusterResourceQuota{
			ObjectMeta: metav1.ObjectMeta{Name: "good"},
			Spec: quotaapi.ClusterResourceQuotaSpec{
				Selector: quotaapi.ClusterResourceQuotaSelector{LabelSelector: &metav1.LabelSelector{MatchLabels: validLabels}},
				Quota:    spec(),
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
				ObjectMeta: metav1.ObjectMeta{Namespace: "bad", Name: "good"},
				Spec: quotaapi.ClusterResourceQuotaSpec{
					Selector: quotaapi.ClusterResourceQuotaSelector{LabelSelector: &metav1.LabelSelector{MatchLabels: validLabels}},
					Quota:    spec(),
				},
			},
			T: field.ErrorTypeForbidden,
			F: "metadata.namespace",
		},
		"missing label selector": {
			A: quotaapi.ClusterResourceQuota{
				ObjectMeta: metav1.ObjectMeta{Name: "good"},
				Spec: quotaapi.ClusterResourceQuotaSpec{
					Quota: spec(),
				},
			},
			T: field.ErrorTypeRequired,
			F: "spec.selector",
		},
		"ok scope": {
			A: quotaapi.ClusterResourceQuota{
				ObjectMeta: metav1.ObjectMeta{Name: "good"},
				Spec: quotaapi.ClusterResourceQuotaSpec{
					Quota: scopeableSpec(api.ResourceQuotaScopeNotTerminating),
				},
			},
			T: field.ErrorTypeRequired,
			F: "spec.selector",
		},
		"bad scope": {
			A: quotaapi.ClusterResourceQuota{
				ObjectMeta: metav1.ObjectMeta{Name: "good"},
				Spec: quotaapi.ClusterResourceQuotaSpec{
					Selector: quotaapi.ClusterResourceQuotaSelector{LabelSelector: &metav1.LabelSelector{MatchLabels: validLabels}},
					Quota:    spec(api.ResourceQuotaScopeNotTerminating),
				},
			},
			T: field.ErrorTypeInvalid,
			F: "spec.quota.scopes",
		},
		"bad quota spec": {
			A: quotaapi.ClusterResourceQuota{
				ObjectMeta: metav1.ObjectMeta{Name: "good"},
				Spec: quotaapi.ClusterResourceQuotaSpec{
					Selector: quotaapi.ClusterResourceQuotaSelector{LabelSelector: &metav1.LabelSelector{MatchLabels: validLabels}},
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

func TestValidationQuota(t *testing.T) {
	tests := map[string]struct {
		A api.ResourceQuota
		T field.ErrorType
		F string
	}{
		"scope": {
			A: api.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "good"},
				Spec:       scopeableSpec(api.ResourceQuotaScopeNotTerminating),
			},
		},
	}
	for k, v := range tests {
		errs := validation.ValidateResourceQuota(&v.A)
		if len(errs) != 0 {
			t.Errorf("%s: %v", k, errs)
			continue
		}
	}
}
