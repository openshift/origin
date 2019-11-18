package migrators

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	openapi_v2 "github.com/googleapis/gnostic/OpenAPIv2"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
)

func TestInProcessMigrator(t *testing.T) {
	apiResources := []metav1.APIResource{
		{
			Name:       "secrets",
			Namespaced: true,
			Group:      "",
			Version:    "v1",
		},
		{
			Name:       "configmaps",
			Namespaced: true,
			Group:      "",
			Version:    "v1",
		},
	}
	grs := []schema.GroupResource{
		{Resource: "configmaps"},
		{Resource: "secrets"},
	}

	tests := []struct {
		name      string
		resources []runtime.Object
	}{
		{
			name:      "no resources",
			resources: nil,
		},
		{
			name: "secrets and configmaps",
			resources: []runtime.Object{
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: "ns1"}},
				&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: "ns2"}},
				&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "ns1"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset()

			scheme := runtime.NewScheme()
			unstructuredObjs := []runtime.Object{}
			for _, rawObject := range tt.resources {
				rawUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rawObject.DeepCopyObject())
				if err != nil {
					t.Fatal(err)
				}
				unstructured.SetNestedField(rawUnstructured, "v1", "apiVersion")
				unstructured.SetNestedField(rawUnstructured, reflect.TypeOf(rawObject).Elem().Name(), "kind")
				unstructuredObjs = append(unstructuredObjs, &unstructured.Unstructured{Object: rawUnstructured})
			}
			dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, unstructuredObjs...)

			discoveryClient := &fakeDisco{
				delegate: fakeKubeClient.Discovery(),
				serverPreferredRes: []*metav1.APIResourceList{
					{
						TypeMeta:     metav1.TypeMeta{},
						APIResources: apiResources,
					},
				},
			}

			handler := &fakeHandler{}

			m := NewInProcessMigrator(dynamicClient, discoveryClient)
			m.AddEventHandler(handler)

			t.Logf("Pruning non-existing migration")
			if err := m.PruneMigration(schema.GroupResource{Resource: "configmaps"}); err != nil {
				t.Errorf("unexpected prune error: %v", err)
			}

			t.Logf("Migrating configmaps")
			err := wait.PollImmediate(100*time.Millisecond, wait.ForeverTestTimeout, func() (bool, error) {
				allFinished := true
				for _, gr := range grs {
					finished, result, _, err := m.EnsureMigration(gr, "1")
					if err != nil {
						return false, err
					}
					if finished && result != nil {
						return false, fmt.Errorf("unexpected non-nil result: %v", err)
					}
					if !finished && result != nil {
						return false, fmt.Errorf("result must be nil if not finished, but got: %v", err)
					}
					if !finished {
						allFinished = false
					}
				}
				return allFinished, nil
			})
			if err != nil {
				t.Fatalf("unexpected ensure error: %v", err)
			}

			if reflect.DeepEqual(handler.calls, []string{"update"}) {
				t.Errorf("expected handler update call when finished, but got: %v", handler.calls)
			}

			t.Logf("Pruning finished migration")
			if err := m.PruneMigration(schema.GroupResource{Resource: "configmaps"}); err != nil {
				t.Errorf("unexpected prune error: %v", err)
			}

			validateMigratedResources(t, dynamicClient.Actions(), unstructuredObjs, grs)
		})
	}
}

func validateMigratedResources(ts *testing.T, actions []clientgotesting.Action, unstructuredObjs []runtime.Object, targetGRs []schema.GroupResource) {
	ts.Helper()

	expectedActionsNoList := len(actions) - len(targetGRs) // subtract "list" requests
	if expectedActionsNoList != len(unstructuredObjs) {
		ts.Fatalf("incorrect number of resources were encrypted, expected %d, got %d", len(unstructuredObjs), expectedActionsNoList)
	}

	// validate LIST requests
	{
		validatedListRequests := 0
		for _, gr := range targetGRs {
			for _, action := range actions {
				if action.Matches("list", gr.Resource) {
					validatedListRequests++
					break
				}
			}
		}
		if validatedListRequests != len(targetGRs) {
			ts.Fatalf("incorrect number of LIST request, expedted %d, got %d", len(targetGRs), validatedListRequests)
		}
	}

	// validate UPDATE requests
	for _, action := range actions {
		if action.GetVerb() == "update" {
			unstructuredObjValidated := false

			updateAction := action.(clientgotesting.UpdateAction)
			updatedObj := updateAction.GetObject().(*unstructured.Unstructured)
			for _, rawUnstructuredObj := range unstructuredObjs {
				expectedUnstructuredObj, ok := rawUnstructuredObj.(*unstructured.Unstructured)
				if !ok {
					ts.Fatalf("object %T is not *unstructured.Unstructured", expectedUnstructuredObj)
				}
				if equality.Semantic.DeepEqual(updatedObj, expectedUnstructuredObj) {
					unstructuredObjValidated = true
					break
				}
			}

			if !unstructuredObjValidated {
				ts.Fatalf("encrypted object with kind = %s, namespace = %s and name = %s wasn't expected to be encrypted", updatedObj.GetKind(), updatedObj.GetNamespace(), updatedObj.GetName())
			}
		}
	}
}

type fakeHandler struct {
	calls []string
}

func (h *fakeHandler) OnAdd(obj interface{}) {
	h.calls = append(h.calls, "add")
}

func (h *fakeHandler) OnUpdate(oldObj, newObj interface{}) {
	h.calls = append(h.calls, "update")
}

func (h *fakeHandler) OnDelete(obj interface{}) {
	h.calls = append(h.calls, "delete")
}

type fakeDisco struct {
	delegate           discovery.DiscoveryInterface
	serverPreferredRes []*metav1.APIResourceList
}

func (f *fakeDisco) RESTClient() interface{} {
	return f.delegate
}

func (f *fakeDisco) ServerGroups() (*metav1.APIGroupList, error) {
	return f.delegate.ServerGroups()
}

func (f *fakeDisco) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	return f.delegate.ServerResourcesForGroupVersion(groupVersion)
}

func (f *fakeDisco) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return f.delegate.ServerGroupsAndResources()
}

func (f *fakeDisco) ServerResources() ([]*metav1.APIResourceList, error) {
	return f.delegate.ServerResources()
}

func (f *fakeDisco) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return f.serverPreferredRes, nil
}

func (f *fakeDisco) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return f.delegate.ServerPreferredNamespacedResources()
}

func (f *fakeDisco) ServerVersion() (*version.Info, error) {
	return f.delegate.ServerVersion()
}

func (f *fakeDisco) OpenAPISchema() (*openapi_v2.Document, error) {
	return f.delegate.OpenAPISchema()
}
