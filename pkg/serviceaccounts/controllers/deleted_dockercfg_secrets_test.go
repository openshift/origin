package controllers

import (
	"math/rand"
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/runtime"
)

func TestDockercfgDeletion(t *testing.T) {
	testcases := map[string]struct {
		ClientObjects []runtime.Object

		DeletedSecret *api.Secret

		ExpectedActions []core.Action
	}{
		"deleted dockercfg secret without serviceaccount": {
			DeletedSecret: createdDockercfgSecret(),

			ExpectedActions: []core.Action{
				core.NewGetAction(unversioned.GroupVersionResource{Resource: "serviceaccounts"}, "default", "default"),
				core.NewDeleteAction(unversioned.GroupVersionResource{Resource: "secrets"}, "default", "token-secret-1"),
			},
		},
		"deleted dockercfg secret with serviceaccount with reference": {
			ClientObjects: []runtime.Object{serviceAccount(addTokenSecretReference(tokenSecretReferences()), imagePullSecretReferences()), createdDockercfgSecret()},

			DeletedSecret: createdDockercfgSecret(),
			ExpectedActions: []core.Action{
				core.NewGetAction(unversioned.GroupVersionResource{Resource: "serviceaccounts"}, "default", "default"),
				core.NewUpdateAction(unversioned.GroupVersionResource{Resource: "serviceaccounts"}, "default", serviceAccount(tokenSecretReferences(), emptyImagePullSecretReferences())),
				core.NewDeleteAction(unversioned.GroupVersionResource{Resource: "secrets"}, "default", "token-secret-1"),
			},
		},
		"deleted dockercfg secret with serviceaccount without reference": {
			ClientObjects: []runtime.Object{serviceAccount(addTokenSecretReference(tokenSecretReferences()), imagePullSecretReferences()), createdDockercfgSecret()},

			DeletedSecret: createdDockercfgSecret(),
			ExpectedActions: []core.Action{
				core.NewGetAction(unversioned.GroupVersionResource{Resource: "serviceaccounts"}, "default", "default"),
				core.NewUpdateAction(unversioned.GroupVersionResource{Resource: "serviceaccounts"}, "default", serviceAccount(tokenSecretReferences(), emptyImagePullSecretReferences())),
				core.NewDeleteAction(unversioned.GroupVersionResource{Resource: "secrets"}, "default", "token-secret-1"),
			},
		},
	}

	for k, tc := range testcases {
		// Re-seed to reset name generation
		rand.Seed(1)

		client := fake.NewSimpleClientset(tc.ClientObjects...)

		controller := NewDockercfgDeletedController(client, DockercfgDeletedControllerOptions{})

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
	}
}
