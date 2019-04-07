package clusterquotareconciliation

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utildiff "k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	clientgotesting "k8s.io/client-go/testing"
	utilquota "k8s.io/kubernetes/pkg/quota/v1"

	quotav1 "github.com/openshift/api/quota/v1"
	fakequotaclient "github.com/openshift/client-go/quota/clientset/versioned/fake"
	quotav1conversions "github.com/openshift/origin/pkg/quota/apis/quota/v1"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
)

func defaultQuota() *quotav1.ClusterResourceQuota {
	return &quotav1.ClusterResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: quotav1.ClusterResourceQuotaSpec{
			Quota: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					corev1.ResourcePods:    resource.MustParse("10"),
					corev1.ResourceSecrets: resource.MustParse("5"),
				},
			},
		},
	}
}

func TestSyncFunc(t *testing.T) {
	testCases := []struct {
		name            string
		startingQuota   func() *quotav1.ClusterResourceQuota
		workItems       []workItem
		mapperFunc      func() clusterquotamapping.ClusterQuotaMapper
		calculationFunc func(namespaceName string, scopes []corev1.ResourceQuotaScope, hardLimits corev1.ResourceList, registry utilquota.Registry, scopeSelector *corev1.ScopeSelector) (corev1.ResourceList, error)

		expectedQuota   func() *quotav1.ClusterResourceQuota
		expectedRetries []workItem
		expectedError   string
	}{
		{
			name:          "from nothing",
			startingQuota: defaultQuota,
			workItems: []workItem{
				{namespaceName: "one"},
			},
			mapperFunc: func() clusterquotamapping.ClusterQuotaMapper {
				mapper := newFakeClusterQuotaMapper()
				mapper.quotaToNamespaces["foo"] = sets.NewString("one")
				return mapper
			},
			calculationFunc: func(namespaceName string, scopes []corev1.ResourceQuotaScope, hardLimits corev1.ResourceList, registry utilquota.Registry, scopeSelector *corev1.ScopeSelector) (corev1.ResourceList, error) {
				if e, a := "one", namespaceName; e != a {
					t.Errorf("%s: expected %v, got %v", "from nothing", e, a)
				}
				ret := corev1.ResourceList{}
				ret[corev1.ResourcePods] = resource.MustParse("10")
				return ret, nil
			},
			expectedQuota: func() *quotav1.ClusterResourceQuota {
				ret := defaultQuota()
				ret.Status.Total.Hard = ret.Spec.Quota.Hard
				ret.Status.Total.Used = corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")}
				quotav1conversions.InsertResourceQuotasStatus(&ret.Status.Namespaces, quotav1.ResourceQuotaStatusByNamespace{
					Namespace: "one",
					Status: corev1.ResourceQuotaStatus{
						Hard: ret.Spec.Quota.Hard,
						Used: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")},
					},
				})
				return ret
			},
			expectedRetries: []workItem{},
		},
		{
			name:          "cache not ready",
			startingQuota: defaultQuota,
			workItems: []workItem{
				{namespaceName: "one"},
			},
			mapperFunc: func() clusterquotamapping.ClusterQuotaMapper {
				mapper := newFakeClusterQuotaMapper()
				mapper.quotaToNamespaces["foo"] = sets.NewString("one")
				mapper.quotaToSelector["foo"] = quotav1.ClusterResourceQuotaSelector{LabelSelector: &metav1.LabelSelector{}}
				return mapper
			},
			calculationFunc: func(namespaceName string, scopes []corev1.ResourceQuotaScope, hardLimits corev1.ResourceList, registry utilquota.Registry, scopeSelector *corev1.ScopeSelector) (corev1.ResourceList, error) {
				t.Errorf("%s: shouldn't be called", "cache not ready")
				return nil, nil
			},
			expectedQuota: func() *quotav1.ClusterResourceQuota {
				return nil
			},
			expectedRetries: []workItem{
				{namespaceName: "one"},
			},
			expectedError: "mapping not up to date",
		},
		{
			name:          "removed from nothing",
			startingQuota: defaultQuota,
			workItems: []workItem{
				{namespaceName: "one"},
			},
			mapperFunc: func() clusterquotamapping.ClusterQuotaMapper {
				return newFakeClusterQuotaMapper()
			},
			calculationFunc: func(namespaceName string, scopes []corev1.ResourceQuotaScope, hardLimits corev1.ResourceList, registry utilquota.Registry, scopeSelector *corev1.ScopeSelector) (corev1.ResourceList, error) {
				if e, a := "one", namespaceName; e != a {
					t.Errorf("%s: expected %v, got %v", "removed from nothing", e, a)
				}
				ret := corev1.ResourceList{}
				ret[corev1.ResourcePods] = resource.MustParse("10")
				return ret, nil
			},
			expectedQuota: func() *quotav1.ClusterResourceQuota {
				ret := defaultQuota()
				ret.Status.Total.Hard = ret.Spec.Quota.Hard
				ret.Status.Total.Used = corev1.ResourceList{}
				return ret
			},
			expectedRetries: []workItem{},
		},
		{
			name: "removed from something",
			startingQuota: func() *quotav1.ClusterResourceQuota {
				ret := defaultQuota()
				ret.Status.Total.Hard = ret.Spec.Quota.Hard
				ret.Status.Total.Used = corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")}
				quotav1conversions.InsertResourceQuotasStatus(&ret.Status.Namespaces, quotav1.ResourceQuotaStatusByNamespace{
					Namespace: "one",
					Status: corev1.ResourceQuotaStatus{
						Hard: ret.Spec.Quota.Hard,
						Used: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")},
					},
				})
				return ret
			},
			workItems: []workItem{
				{namespaceName: "one"},
			},
			mapperFunc: func() clusterquotamapping.ClusterQuotaMapper {
				return newFakeClusterQuotaMapper()
			},
			calculationFunc: func(namespaceName string, scopes []corev1.ResourceQuotaScope, hardLimits corev1.ResourceList, registry utilquota.Registry, scopeSelector *corev1.ScopeSelector) (corev1.ResourceList, error) {
				if e, a := "one", namespaceName; e != a {
					t.Errorf("%s: expected %v, got %v", "removed from something", e, a)
				}
				ret := corev1.ResourceList{}
				ret[corev1.ResourcePods] = resource.MustParse("10")
				return ret, nil
			},
			expectedQuota: func() *quotav1.ClusterResourceQuota {
				ret := defaultQuota()
				ret.Status.Total.Hard = ret.Spec.Quota.Hard
				ret.Status.Total.Used = corev1.ResourceList{corev1.ResourcePods: resource.MustParse("0")}
				return ret
			},
			expectedRetries: []workItem{},
		},
		{
			name: "update one, remove two, ignore three, fail four, remove deleted",
			startingQuota: func() *quotav1.ClusterResourceQuota {
				ret := defaultQuota()
				ret.Status.Total.Hard = ret.Spec.Quota.Hard
				ret.Status.Total.Used = corev1.ResourceList{corev1.ResourcePods: resource.MustParse("30")}
				quotav1conversions.InsertResourceQuotasStatus(&ret.Status.Namespaces, quotav1.ResourceQuotaStatusByNamespace{
					Namespace: "one",
					Status: corev1.ResourceQuotaStatus{
						Hard: ret.Spec.Quota.Hard,
						Used: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("5")},
					},
				})
				quotav1conversions.InsertResourceQuotasStatus(&ret.Status.Namespaces, quotav1.ResourceQuotaStatusByNamespace{
					Namespace: "two",
					Status: corev1.ResourceQuotaStatus{
						Hard: ret.Spec.Quota.Hard,
						Used: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")},
					},
				})
				quotav1conversions.InsertResourceQuotasStatus(&ret.Status.Namespaces, quotav1.ResourceQuotaStatusByNamespace{
					Namespace: "three",
					Status: corev1.ResourceQuotaStatus{
						Hard: ret.Spec.Quota.Hard,
						Used: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("15")},
					},
				})
				quotav1conversions.InsertResourceQuotasStatus(&ret.Status.Namespaces, quotav1.ResourceQuotaStatusByNamespace{
					Namespace: "deleted",
					Status: corev1.ResourceQuotaStatus{
						Hard: ret.Spec.Quota.Hard,
						Used: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("0")},
					},
				})
				return ret
			},
			workItems: []workItem{
				{namespaceName: "one", forceRecalculation: true},
				{namespaceName: "two"},
				{namespaceName: "three"},
				{namespaceName: "four"},
			},
			mapperFunc: func() clusterquotamapping.ClusterQuotaMapper {
				mapper := newFakeClusterQuotaMapper()
				mapper.quotaToNamespaces["foo"] = sets.NewString("one", "three", "four")
				return mapper
			},
			calculationFunc: func(namespaceName string, scopes []corev1.ResourceQuotaScope, hardLimits corev1.ResourceList, registry utilquota.Registry, scopeSelector *corev1.ScopeSelector) (corev1.ResourceList, error) {
				if namespaceName == "four" {
					return nil, fmt.Errorf("calculation error")
				}
				ret := corev1.ResourceList{}
				ret[corev1.ResourcePods] = resource.MustParse("10")
				return ret, nil
			},
			expectedQuota: func() *quotav1.ClusterResourceQuota {
				ret := defaultQuota()
				ret.Status.Total.Hard = ret.Spec.Quota.Hard
				ret.Status.Total.Used = corev1.ResourceList{corev1.ResourcePods: resource.MustParse("25")}
				quotav1conversions.InsertResourceQuotasStatus(&ret.Status.Namespaces, quotav1.ResourceQuotaStatusByNamespace{
					Namespace: "one",
					Status: corev1.ResourceQuotaStatus{
						Hard: ret.Spec.Quota.Hard,
						Used: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")},
					},
				})
				quotav1conversions.InsertResourceQuotasStatus(&ret.Status.Namespaces, quotav1.ResourceQuotaStatusByNamespace{
					Namespace: "three",
					Status: corev1.ResourceQuotaStatus{
						Hard: ret.Spec.Quota.Hard,
						Used: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("15")},
					},
				})
				return ret
			},
			expectedRetries: []workItem{{namespaceName: "four"}},
			expectedError:   "calculation error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := fakequotaclient.NewSimpleClientset(tc.startingQuota())

			quotaUsageCalculationFunc = tc.calculationFunc
			// we only need these fields to test the sync func
			controller := ClusterQuotaReconcilationController{
				clusterQuotaMapper: tc.mapperFunc(),
				clusterQuotaClient: client.QuotaV1().ClusterResourceQuotas(),
			}

			actualErr, actualRetries := controller.syncQuotaForNamespaces(tc.startingQuota(), tc.workItems)
			switch {
			case len(tc.expectedError) == 0 && actualErr == nil:
			case len(tc.expectedError) == 0 && actualErr != nil:
				t.Fatalf("%s: unexpected error: %v", tc.name, actualErr)
			case len(tc.expectedError) != 0 && actualErr == nil:
				t.Fatalf("%s: missing expected error: %v", tc.name, tc.expectedError)
			case len(tc.expectedError) != 0 && actualErr != nil && !strings.Contains(actualErr.Error(), tc.expectedError):
				t.Fatalf("%s: expected %v, got %v", tc.name, tc.expectedError, actualErr)
			}

			if !reflect.DeepEqual(actualRetries, tc.expectedRetries) {
				t.Fatalf("%s: expected %v, got %v", tc.name, tc.expectedRetries, actualRetries)
			}

			var actualQuota *quotav1.ClusterResourceQuota
			for _, action := range client.Actions() {
				updateAction, ok := action.(clientgotesting.UpdateActionImpl)
				if !ok {
					continue
				}
				if updateAction.Matches("update", "clusterresourcequotas") && updateAction.Subresource == "status" {
					actualQuota = updateAction.GetObject().(*quotav1.ClusterResourceQuota)
					break
				}
			}

			if tc.expectedQuota() == nil && actualQuota == nil {
				return
			}

			if tc.expectedQuota() == nil && actualQuota != nil {
				t.Fatalf("%s: expected %v, got %v", tc.name, "nil", actualQuota)
			}

			if !equality.Semantic.DeepEqual(tc.expectedQuota(), actualQuota) {
				t.Fatalf("%s: %v", tc.name, utildiff.ObjectDiff(tc.expectedQuota(), actualQuota))
			}
		})
	}

}

type fakeClusterQuotaMapper struct {
	quotaToSelector            map[string]quotav1.ClusterResourceQuotaSelector
	namespaceToSelectionFields map[string]clusterquotamapping.SelectionFields

	quotaToNamespaces map[string]sets.String
	namespaceToQuota  map[string]sets.String
}

func newFakeClusterQuotaMapper() *fakeClusterQuotaMapper {
	return &fakeClusterQuotaMapper{
		quotaToSelector:            map[string]quotav1.ClusterResourceQuotaSelector{},
		namespaceToSelectionFields: map[string]clusterquotamapping.SelectionFields{},
		quotaToNamespaces:          map[string]sets.String{},
		namespaceToQuota:           map[string]sets.String{},
	}
}

func (m *fakeClusterQuotaMapper) GetClusterQuotasFor(namespaceName string) ([]string, clusterquotamapping.SelectionFields) {
	return m.namespaceToQuota[namespaceName].List(), m.namespaceToSelectionFields[namespaceName]
}
func (m *fakeClusterQuotaMapper) GetNamespacesFor(quotaName string) ([]string, quotav1.ClusterResourceQuotaSelector) {
	return m.quotaToNamespaces[quotaName].List(), m.quotaToSelector[quotaName]
}
func (m *fakeClusterQuotaMapper) AddListener(listener clusterquotamapping.MappingChangeListener) {}
