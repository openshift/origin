package defaultrolebindings

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeclientfake "k8s.io/client-go/kubernetes/fake"
	corelisters "k8s.io/client-go/listers/core/v1"
	rbaclisters "k8s.io/client-go/listers/rbac/v1"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/controller"
)

func TestSync(t *testing.T) {
	tests := []struct {
		name                      string
		startingNamespaces        []*corev1.Namespace
		startingRoleBindings      []*rbacv1.RoleBinding
		namespaceToSync           string
		expectedRoleBindingsNames []string
	}{
		{
			name: "create-all",
			startingNamespaces: []*corev1.Namespace{
				{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
			},
			startingRoleBindings: []*rbacv1.RoleBinding{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "bar"}},
			},
			namespaceToSync:           "foo",
			expectedRoleBindingsNames: []string{"system:image-pullers", "system:image-builders", "system:deployers"},
		},
		{
			name: "create-missing",
			startingNamespaces: []*corev1.Namespace{
				{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
			},
			startingRoleBindings: []*rbacv1.RoleBinding{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "bar"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "system:image-builders"}},
			},
			namespaceToSync:           "foo",
			expectedRoleBindingsNames: []string{"system:image-pullers", "system:deployers"},
		},
		{
			name: "create-none",
			startingNamespaces: []*corev1.Namespace{
				{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
			},
			startingRoleBindings: []*rbacv1.RoleBinding{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "system:image-builders"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "system:image-pullers"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "system:deployers"}},
			},
			namespaceToSync: "foo",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			roleBindingIndexer := cache.NewIndexer(controller.KeyFunc, cache.Indexers{})
			namespaceIndexer := cache.NewIndexer(controller.KeyFunc, cache.Indexers{})
			objs := []runtime.Object{}
			for _, obj := range test.startingRoleBindings {
				objs = append(objs, obj)
				roleBindingIndexer.Add(obj)
			}
			for _, obj := range test.startingNamespaces {
				objs = append(objs, obj)
				namespaceIndexer.Add(obj)
			}
			fakeClient := kubeclientfake.NewSimpleClientset(objs...)
			c := DefaultRoleBindingController{
				roleBindingClient: fakeClient.RbacV1(),
				roleBindingLister: rbaclisters.NewRoleBindingLister(roleBindingIndexer),
				namespaceLister:   corelisters.NewNamespaceLister(namespaceIndexer),
			}

			err := c.syncNamespace(test.namespaceToSync)
			if err != nil {
				t.Fatal(err)
			}

			allActions := fakeClient.Actions()
			createActions := []clienttesting.CreateAction{}
			for i := range allActions {
				action := allActions[i]
				createAction, ok := action.(clienttesting.CreateAction)
				if !ok {
					t.Errorf("unexpected action %#v", action)
				}
				createActions = append(createActions, createAction)
			}
			if len(createActions) != len(test.expectedRoleBindingsNames) {
				t.Fatalf("expected %v, got %#v", test.expectedRoleBindingsNames, createActions)
			}

			for i, name := range test.expectedRoleBindingsNames {
				action := createActions[i]
				metadata, err := meta.Accessor(action.GetObject())
				if err != nil {
					t.Fatal(err)
				}
				if name != metadata.GetName() {
					t.Errorf("expected %v, got %v", name, metadata.GetName())
				}
				if action.GetNamespace() != test.namespaceToSync {
					t.Errorf("expected %v, got %v", test.namespaceToSync, action.GetNamespace())
				}
			}
		})
	}

}
