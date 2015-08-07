package controllers

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

func TestDockercfgDeletion(t *testing.T) {
	testcases := map[string]struct {
		ClientObjects []runtime.Object

		DeletedSecret *api.Secret

		ExpectedActions []testclient.FakeAction
	}{
		"deleted dockercfg secret without serviceaccount": {
			DeletedSecret: createdDockercfgSecret(),

			ExpectedActions: []testclient.FakeAction{
				{Action: "get-serviceaccount", Value: "default"},
				{Action: "delete-secret", Value: "token-secret-1"},
			},
		},
		"deleted dockercfg secret with serviceaccount with reference": {
			ClientObjects: []runtime.Object{serviceAccount(addTokenSecretReference(tokenSecretReferences()), imagePullSecretReferences()), createdDockercfgSecret()},

			DeletedSecret: createdDockercfgSecret(),
			ExpectedActions: []testclient.FakeAction{
				{Action: "get-serviceaccount", Value: "default"},
				{Action: "update-serviceaccount", Value: serviceAccount(tokenSecretReferences(), emptyImagePullSecretReferences())},
				{Action: "delete-secret", Value: "token-secret-1"},
			},
		},
		"deleted dockercfg secret with serviceaccount without reference": {
			ClientObjects: []runtime.Object{serviceAccount(addTokenSecretReference(tokenSecretReferences()), imagePullSecretReferences()), createdDockercfgSecret()},

			DeletedSecret: createdDockercfgSecret(),
			ExpectedActions: []testclient.FakeAction{
				{Action: "get-serviceaccount", Value: "default"},
				{Action: "update-serviceaccount", Value: serviceAccount(tokenSecretReferences(), emptyImagePullSecretReferences())},
				{Action: "delete-secret", Value: "token-secret-1"},
			},
		},
	}

	for k, tc := range testcases {
		// Re-seed to reset name generation
		rand.Seed(1)

		client := testclient.NewSimpleFake(tc.ClientObjects...)

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
			if expectedAction.Action != action.Action {
				t.Errorf("%s: Expected %s, got %s", k, expectedAction.Action, action.Action)
				continue
			}
			if !reflect.DeepEqual(expectedAction.Value, action.Value) {
				t.Errorf("%s: Expected\n\t%#v\ngot\n\t%#v", k, expectedAction.Value, action.Value)
				continue
			}
		}

		if len(tc.ExpectedActions) > len(client.Actions()) {
			t.Errorf("%s: %d additional expected actions:%+v", k, len(tc.ExpectedActions)-len(client.Actions()), tc.ExpectedActions[len(client.Actions()):])
		}
	}
}
