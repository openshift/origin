package clusterresourcequota

import (
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/client/cache"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	utildiff "k8s.io/kubernetes/pkg/util/diff"
	"k8s.io/kubernetes/pkg/util/sets"

	ocache "github.com/openshift/origin/pkg/client/cache"
	"github.com/openshift/origin/pkg/client/testclient"
	quotaapi "github.com/openshift/origin/pkg/quota/api"
	quotaapiv1 "github.com/openshift/origin/pkg/quota/api/v1"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
)

func TestUpdateQuota(t *testing.T) {
	testCases := []struct {
		name            string
		availableQuotas func() []*quotaapi.ClusterResourceQuota
		quotaToUpdate   *kapi.ResourceQuota

		expectedQuota func() *quotaapi.ClusterResourceQuota
		expectedError string
	}{
		{
			name: "update properly",
			availableQuotas: func() []*quotaapi.ClusterResourceQuota {
				user1 := defaultQuota()
				user1.Name = "user-one"
				user1.Status.Total.Hard = user1.Spec.Quota.Hard
				user1.Status.Total.Used = kapi.ResourceList{kapi.ResourcePods: resource.MustParse("15")}
				user1.Status.Namespaces.Insert("foo", kapi.ResourceQuotaStatus{
					Hard: user1.Spec.Quota.Hard,
					Used: kapi.ResourceList{kapi.ResourcePods: resource.MustParse("5")},
				})
				user1.Status.Namespaces.Insert("bar", kapi.ResourceQuotaStatus{
					Hard: user1.Spec.Quota.Hard,
					Used: kapi.ResourceList{kapi.ResourcePods: resource.MustParse("10")},
				})

				user2 := defaultQuota()
				user2.Name = "user-two"
				user2.Status.Total.Hard = user2.Spec.Quota.Hard
				user2.Status.Total.Used = kapi.ResourceList{kapi.ResourcePods: resource.MustParse("5")}
				user2.Status.Namespaces.Insert("foo", kapi.ResourceQuotaStatus{
					Hard: user2.Spec.Quota.Hard,
					Used: kapi.ResourceList{kapi.ResourcePods: resource.MustParse("5")},
				})

				return []*quotaapi.ClusterResourceQuota{user1, user2}
			},
			quotaToUpdate: &kapi.ResourceQuota{
				ObjectMeta: kapi.ObjectMeta{Namespace: "foo", Name: "user-one"},
				Spec: kapi.ResourceQuotaSpec{
					Hard: kapi.ResourceList{
						kapi.ResourcePods:    resource.MustParse("10"),
						kapi.ResourceSecrets: resource.MustParse("5"),
					},
				},
				Status: kapi.ResourceQuotaStatus{
					Hard: kapi.ResourceList{
						kapi.ResourcePods:    resource.MustParse("10"),
						kapi.ResourceSecrets: resource.MustParse("5"),
					},
					Used: kapi.ResourceList{
						kapi.ResourcePods: resource.MustParse("20"),
					},
				}},

			expectedQuota: func() *quotaapi.ClusterResourceQuota {
				user1 := defaultQuota()
				user1.Name = "user-one"
				user1.Status.Total.Hard = user1.Spec.Quota.Hard
				user1.Status.Total.Used = kapi.ResourceList{kapi.ResourcePods: resource.MustParse("20")}
				user1.Status.Namespaces.Insert("foo", kapi.ResourceQuotaStatus{
					Hard: user1.Spec.Quota.Hard,
					Used: kapi.ResourceList{kapi.ResourcePods: resource.MustParse("10")},
				})
				user1.Status.Namespaces.Insert("bar", kapi.ResourceQuotaStatus{
					Hard: user1.Spec.Quota.Hard,
					Used: kapi.ResourceList{kapi.ResourcePods: resource.MustParse("10")},
				})

				return user1
			},
		},
	}

	for _, tc := range testCases {
		quotaIndexer := cache.NewIndexer(framework.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{})
		availableQuotas := tc.availableQuotas()
		objs := []runtime.Object{}
		for i := range availableQuotas {
			quotaIndexer.Add(availableQuotas[i])
			objs = append(objs, availableQuotas[i])
		}
		quotaLister := &ocache.IndexerToClusterResourceQuotaLister{Indexer: quotaIndexer}

		client := testclient.NewSimpleFake(objs...)

		accessor := newQuotaAccessor(quotaLister, nil, client, nil)

		actualErr := accessor.UpdateQuotaStatus(tc.quotaToUpdate)
		switch {
		case len(tc.expectedError) == 0 && actualErr == nil:
		case len(tc.expectedError) == 0 && actualErr != nil:
			t.Errorf("%s: unexpected error: %v", tc.name, actualErr)
			continue
		case len(tc.expectedError) != 0 && actualErr == nil:
			t.Errorf("%s: missing expected error: %v", tc.name, tc.expectedError)
			continue
		case len(tc.expectedError) != 0 && actualErr != nil && !strings.Contains(actualErr.Error(), tc.expectedError):
			t.Errorf("%s: expected %v, got %v", tc.name, tc.expectedError, actualErr)
			continue
		}

		var actualQuota *quotaapi.ClusterResourceQuota
		for _, action := range client.Actions() {
			updateAction, ok := action.(ktestclient.UpdateActionImpl)
			if !ok {
				continue
			}
			if updateAction.Matches("update", "clusterresourcequotas") && updateAction.Subresource == "status" {
				actualQuota = updateAction.GetObject().(*quotaapi.ClusterResourceQuota)
				break
			}
		}

		expectedV1, err := kapi.Scheme.ConvertToVersion(tc.expectedQuota(), quotaapiv1.SchemeGroupVersion)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		actualV1, err := kapi.Scheme.ConvertToVersion(actualQuota, quotaapiv1.SchemeGroupVersion)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if !kapi.Semantic.DeepEqual(expectedV1, actualV1) {
			t.Errorf("%s: %v", tc.name, utildiff.ObjectDiff(expectedV1, actualV1))
			continue
		}
	}

}

func defaultQuota() *quotaapi.ClusterResourceQuota {
	return &quotaapi.ClusterResourceQuota{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
		Spec: quotaapi.ClusterResourceQuotaSpec{
			Quota: kapi.ResourceQuotaSpec{
				Hard: kapi.ResourceList{
					kapi.ResourcePods:    resource.MustParse("10"),
					kapi.ResourceSecrets: resource.MustParse("5"),
				},
			},
		},
	}
}

func TestGetQuota(t *testing.T) {
	testCases := []struct {
		name                string
		availableQuotas     func() []*quotaapi.ClusterResourceQuota
		availableNamespaces []*kapi.Namespace
		mapperFunc          func() clusterquotamapping.ClusterQuotaMapper
		requestedNamespace  string

		expectedQuotas func() []*kapi.ResourceQuota
		expectedError  string
	}{
		{
			name: "namespace not synced",
			availableQuotas: func() []*quotaapi.ClusterResourceQuota {
				return nil
			},
			availableNamespaces: []*kapi.Namespace{
				{ObjectMeta: kapi.ObjectMeta{Name: "foo", Labels: map[string]string{"one": "alfa"}}},
			},
			mapperFunc: func() clusterquotamapping.ClusterQuotaMapper {
				mapper := newFakeClusterQuotaMapper()
				mapper.namespaceToQuota["foo"] = sets.NewString("user-one")
				mapper.namespaceToSelectionFields["foo"] = clusterquotamapping.SelectionFields{Labels: map[string]string{"two": "bravo"}}
				return mapper
			},
			requestedNamespace: "foo",

			expectedError: "timed out waiting for the condition",
		},
		{
			name: "no hits on namespace",
			availableQuotas: func() []*quotaapi.ClusterResourceQuota {
				return nil
			},
			availableNamespaces: []*kapi.Namespace{
				{ObjectMeta: kapi.ObjectMeta{Name: "foo", Labels: map[string]string{"one": "alfa"}}},
			},
			mapperFunc: func() clusterquotamapping.ClusterQuotaMapper {
				mapper := newFakeClusterQuotaMapper()
				mapper.namespaceToQuota["foo"] = sets.NewString()
				mapper.namespaceToSelectionFields["foo"] = clusterquotamapping.SelectionFields{Labels: map[string]string{"one": "alfa"}}
				mapper.namespaceToQuota["bar"] = sets.NewString("user-one")
				mapper.namespaceToSelectionFields["bar"] = clusterquotamapping.SelectionFields{Labels: map[string]string{"two": "bravo"}}
				return mapper
			},
			requestedNamespace: "foo",

			expectedQuotas: func() []*kapi.ResourceQuota {
				return []*kapi.ResourceQuota{}
			},
			expectedError: "",
		},
		{
			name: "correct quota and namespaces",
			availableQuotas: func() []*quotaapi.ClusterResourceQuota {
				user1 := defaultQuota()
				user1.Name = "user-one"
				user1.Status.Total.Hard = user1.Spec.Quota.Hard
				user1.Status.Total.Used = kapi.ResourceList{kapi.ResourcePods: resource.MustParse("15")}
				user1.Status.Namespaces.Insert("foo", kapi.ResourceQuotaStatus{
					Hard: user1.Spec.Quota.Hard,
					Used: kapi.ResourceList{kapi.ResourcePods: resource.MustParse("5")},
				})
				user1.Status.Namespaces.Insert("bar", kapi.ResourceQuotaStatus{
					Hard: user1.Spec.Quota.Hard,
					Used: kapi.ResourceList{kapi.ResourcePods: resource.MustParse("10")},
				})

				user2 := defaultQuota()
				user2.Name = "user-two"
				user2.Status.Total.Hard = user2.Spec.Quota.Hard
				user2.Status.Total.Used = kapi.ResourceList{kapi.ResourcePods: resource.MustParse("5")}
				user2.Status.Namespaces.Insert("foo", kapi.ResourceQuotaStatus{
					Hard: user2.Spec.Quota.Hard,
					Used: kapi.ResourceList{kapi.ResourcePods: resource.MustParse("5")},
				})

				return []*quotaapi.ClusterResourceQuota{user1, user2}
			},
			availableNamespaces: []*kapi.Namespace{
				{ObjectMeta: kapi.ObjectMeta{Name: "foo", Labels: map[string]string{"one": "alfa"}}},
			},
			mapperFunc: func() clusterquotamapping.ClusterQuotaMapper {
				mapper := newFakeClusterQuotaMapper()
				mapper.namespaceToQuota["foo"] = sets.NewString("user-one")
				mapper.namespaceToSelectionFields["foo"] = clusterquotamapping.SelectionFields{Labels: map[string]string{"one": "alfa"}}
				return mapper
			},
			requestedNamespace: "foo",

			expectedQuotas: func() []*kapi.ResourceQuota {
				return []*kapi.ResourceQuota{
					{
						ObjectMeta: kapi.ObjectMeta{Namespace: "foo", Name: "user-one"},
						Spec: kapi.ResourceQuotaSpec{
							Hard: kapi.ResourceList{
								kapi.ResourcePods:    resource.MustParse("10"),
								kapi.ResourceSecrets: resource.MustParse("5"),
							},
						},
						Status: kapi.ResourceQuotaStatus{
							Hard: kapi.ResourceList{
								kapi.ResourcePods:    resource.MustParse("10"),
								kapi.ResourceSecrets: resource.MustParse("5"),
							},
							Used: kapi.ResourceList{
								kapi.ResourcePods: resource.MustParse("15"),
							},
						},
					},
				}
			},
		},
	}

	for _, tc := range testCases {
		quotaIndexer := cache.NewIndexer(framework.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{})
		availableQuotas := tc.availableQuotas()
		for i := range availableQuotas {
			quotaIndexer.Add(availableQuotas[i])
		}
		quotaLister := &ocache.IndexerToClusterResourceQuotaLister{Indexer: quotaIndexer}

		namespaceIndexer := cache.NewIndexer(framework.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{})
		for i := range tc.availableNamespaces {
			namespaceIndexer.Add(tc.availableNamespaces[i])
		}
		namespaceLister := &cache.IndexerToNamespaceLister{Indexer: namespaceIndexer}

		client := testclient.NewSimpleFake()

		accessor := newQuotaAccessor(quotaLister, namespaceLister, client, tc.mapperFunc())

		actualQuotas, actualErr := accessor.GetQuotas(tc.requestedNamespace)
		switch {
		case len(tc.expectedError) == 0 && actualErr == nil:
		case len(tc.expectedError) == 0 && actualErr != nil:
			t.Errorf("%s: unexpected error: %v", tc.name, actualErr)
			continue
		case len(tc.expectedError) != 0 && actualErr == nil:
			t.Errorf("%s: missing expected error: %v", tc.name, tc.expectedError)
			continue
		case len(tc.expectedError) != 0 && actualErr != nil && !strings.Contains(actualErr.Error(), tc.expectedError):
			t.Errorf("%s: expected %v, got %v", tc.name, tc.expectedError, actualErr)
			continue
		}

		if tc.expectedQuotas == nil {
			continue
		}

		actualQuotaPointers := []*kapi.ResourceQuota{}
		for i := range actualQuotas {
			actualQuotaPointers = append(actualQuotaPointers, &actualQuotas[i])
		}

		expectedQuotas := tc.expectedQuotas()
		if !kapi.Semantic.DeepEqual(expectedQuotas, actualQuotaPointers) {
			t.Errorf("%s: expectedLen: %v actualLen: %v", tc.name, len(expectedQuotas), len(actualQuotas))
			for i := range expectedQuotas {
				expectedV1, err := kapi.Scheme.ConvertToVersion(expectedQuotas[i], quotaapiv1.SchemeGroupVersion)
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tc.name, err)
					continue
				}
				actualV1, err := kapi.Scheme.ConvertToVersion(actualQuotaPointers[i], quotaapiv1.SchemeGroupVersion)
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tc.name, err)
					continue
				}
				t.Errorf("%s: %v equal? %v", tc.name, utildiff.ObjectDiff(expectedV1, actualV1), kapi.Semantic.DeepEqual(expectedV1, actualV1))
			}
			continue
		}
	}
}

type fakeClusterQuotaMapper struct {
	quotaToSelector            map[string]quotaapi.ClusterResourceQuotaSelector
	namespaceToSelectionFields map[string]clusterquotamapping.SelectionFields

	quotaToNamespaces map[string]sets.String
	namespaceToQuota  map[string]sets.String
}

func newFakeClusterQuotaMapper() *fakeClusterQuotaMapper {
	return &fakeClusterQuotaMapper{
		quotaToSelector:            map[string]quotaapi.ClusterResourceQuotaSelector{},
		namespaceToSelectionFields: map[string]clusterquotamapping.SelectionFields{},
		quotaToNamespaces:          map[string]sets.String{},
		namespaceToQuota:           map[string]sets.String{},
	}
}

func (m *fakeClusterQuotaMapper) GetClusterQuotasFor(namespaceName string) ([]string, clusterquotamapping.SelectionFields) {
	return m.namespaceToQuota[namespaceName].List(), m.namespaceToSelectionFields[namespaceName]
}
func (m *fakeClusterQuotaMapper) GetNamespacesFor(quotaName string) ([]string, quotaapi.ClusterResourceQuotaSelector) {
	return m.quotaToNamespaces[quotaName].List(), m.quotaToSelector[quotaName]
}
func (m *fakeClusterQuotaMapper) AddListener(listener clusterquotamapping.MappingChangeListener) {}
