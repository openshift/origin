package controllers

import (
	"math/rand"
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	informers "k8s.io/client-go/informers"
	externalfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/controller"
)

func TestDockercfgDeletion(t *testing.T) {
	testcases := map[string]struct {
		ClientObjects []runtime.Object

		DeletedSecret *v1.Secret

		ExpectedActions []clientgotesting.Action
	}{
		"deleted dockercfg secret without serviceaccount": {
			DeletedSecret: createdDockercfgSecret(),

			ExpectedActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(schema.GroupVersionResource{Resource: "serviceaccounts", Version: "v1"}, "default", "default"),
				clientgotesting.NewDeleteAction(schema.GroupVersionResource{Resource: "secrets", Version: "v1"}, "default", "token-secret-1"),
			},
		},
		"deleted dockercfg secret with serviceaccount with reference": {
			ClientObjects: []runtime.Object{serviceAccount(addTokenSecretReference(tokenSecretReferences()), imagePullSecretReferences()), createdDockercfgSecret()},

			DeletedSecret: createdDockercfgSecret(),
			ExpectedActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(schema.GroupVersionResource{Resource: "serviceaccounts", Version: "v1"}, "default", "default"),
				clientgotesting.NewUpdateAction(schema.GroupVersionResource{Resource: "serviceaccounts", Version: "v1"}, "default", serviceAccount(tokenSecretReferences(), emptyImagePullSecretReferences())),
				clientgotesting.NewDeleteAction(schema.GroupVersionResource{Resource: "secrets", Version: "v1"}, "default", "token-secret-1"),
			},
		},
		"deleted dockercfg secret with serviceaccount without reference": {
			ClientObjects: []runtime.Object{serviceAccount(addTokenSecretReference(tokenSecretReferences()), imagePullSecretReferences()), createdDockercfgSecret()},

			DeletedSecret: createdDockercfgSecret(),
			ExpectedActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(schema.GroupVersionResource{Resource: "serviceaccounts", Version: "v1"}, "default", "default"),
				clientgotesting.NewUpdateAction(schema.GroupVersionResource{Resource: "serviceaccounts", Version: "v1"}, "default", serviceAccount(tokenSecretReferences(), emptyImagePullSecretReferences())),
				clientgotesting.NewDeleteAction(schema.GroupVersionResource{Resource: "secrets", Version: "v1"}, "default", "token-secret-1"),
			},
		},
	}

	for k, tc := range testcases {
		// Re-seed to reset name generation
		rand.Seed(1)

		client := externalfake.NewSimpleClientset(tc.ClientObjects...)
		informerFactory := informers.NewSharedInformerFactory(client, controller.NoResyncPeriodFunc())
		controller := NewDockercfgDeletedController(
			informerFactory.Core().V1().Secrets(),
			client,
			DockercfgDeletedControllerOptions{},
		)
		stopCh := make(chan struct{})
		informerFactory.Start(stopCh)
		if !cache.WaitForCacheSync(stopCh, controller.secretController.HasSynced) {
			t.Fatalf("unable to reach cache sync")
		}
		client.ClearActions()

		if tc.DeletedSecret != nil {
			controller.secretDeleted(tc.DeletedSecret)
		}

		for i, action := range client.Actions() {
			if len(tc.ExpectedActions) < i+1 {
				t.Errorf("%s: %d unexpected actions: %+v", k, len(client.Actions())-len(tc.ExpectedActions), client.Actions()[i:])
				break
			}

			expectedAction := tc.ExpectedActions[i]
			if !reflect.DeepEqual(expectedAction, action) {
				t.Errorf("%s: Expected %v, got %v", k, expectedAction, action)
				continue
			}
		}

		if len(tc.ExpectedActions) > len(client.Actions()) {
			t.Errorf("%s: %d additional expected actions:%+v", k, len(tc.ExpectedActions)-len(client.Actions()), tc.ExpectedActions[len(client.Actions()):])
		}
		close(stopCh)
	}
}
