package controllers

import (
	"math/rand"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	informers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/controller"
)

// emptySecretReferences is used by a service account without any secrets
func emptySecretReferences() []api.ObjectReference {
	return []api.ObjectReference{}
}

func emptyImagePullSecretReferences() []api.LocalObjectReference {
	return []api.LocalObjectReference{}
}

// missingSecretReferences is used by a service account that references secrets which do no exist
func missingSecretReferences() []api.ObjectReference {
	return []api.ObjectReference{{Name: "missing-secret-1"}}
}

// regularSecretReferences is used by a service account that references secrets which are not ServiceAccountTokens
func regularSecretReferences() []api.ObjectReference {
	return []api.ObjectReference{{Name: "regular-secret-1"}}
}

// tokenSecretReferences is used by a service account that references a ServiceAccountToken secret
func tokenSecretReferences() []api.ObjectReference {
	return []api.ObjectReference{{Name: "token-secret-1"}}
}

// addTokenSecretReference adds a reference to the ServiceAccountToken that will be created
func addTokenSecretReference(refs []api.ObjectReference) []api.ObjectReference {
	return append(refs, api.ObjectReference{Name: "default-dockercfg-fplln"})
}

func imagePullSecretReferences() []api.LocalObjectReference {
	return []api.LocalObjectReference{{Name: "default-dockercfg-fplln"}}
}

// serviceAccount returns a service account with the given secret refs
func serviceAccount(secretRefs []api.ObjectReference, imagePullSecretRefs []api.LocalObjectReference) *api.ServiceAccount {
	return &api.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "default",
			UID:             "12345",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Secrets:          secretRefs,
		ImagePullSecrets: imagePullSecretRefs,
	}
}

// createdDockercfgSecret returns the ServiceAccountToken secret posted when creating a new token secret.
// Named "default-token-fplln", since that is the first generated name after rand.Seed(1)
func createdDockercfgSecret() *api.Secret {
	return &api.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-dockercfg-fplln",
			Namespace: "default",
			Annotations: map[string]string{
				api.ServiceAccountNameKey:        "default",
				api.ServiceAccountUIDKey:         "12345",
				ServiceAccountTokenSecretNameKey: "token-secret-1",
			},
		},
		Type: api.SecretTypeDockercfg,
		Data: map[string][]byte{
			api.DockerConfigKey: []byte(`{"docker-registry.default.svc.cluster.local":{"Username":"serviceaccount","Password":"ABC","Email":"serviceaccount@example.org"}}`),
		},
	}
}

// opaqueSecret returns a persisted non-ServiceAccountToken secret named "regular-secret-1"
func opaqueSecret() *api.Secret {
	return &api.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "regular-secret-1",
			Namespace:       "default",
			UID:             "23456",
			ResourceVersion: "1",
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"mykey": []byte("mydata"),
		},
	}
}

// serviceAccountTokenSecret returns an existing ServiceAccountToken secret named "token-secret-1"
func serviceAccountTokenSecret() *api.Secret {
	return &api.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "token-secret-1",
			Namespace:       "default",
			UID:             "23456",
			ResourceVersion: "1",
			Annotations: map[string]string{
				api.ServiceAccountNameKey: "default",
				api.ServiceAccountUIDKey:  "12345",
			},
		},
		Type: api.SecretTypeServiceAccountToken,
		Data: map[string][]byte{
			"token": []byte("ABC"),
		},
	}
}

// serviceAccountTokenSecretWithoutTokenData returns an existing ServiceAccountToken secret that lacks token data
func serviceAccountTokenSecretWithoutTokenData() *api.Secret {
	secret := serviceAccountTokenSecret()
	secret.Data = nil
	return secret
}

func TestTokenDeletion(t *testing.T) {
	dockercfgSecretFieldSelector := fields.OneTermEqualSelector(api.SecretTypeField, string(api.SecretTypeDockercfg))

	testcases := map[string]struct {
		ClientObjects []runtime.Object

		DeletedSecret *api.Secret

		ExpectedActions []clientgotesting.Action
	}{
		"deleted token secret without serviceaccount": {
			ClientObjects: []runtime.Object{serviceAccount(addTokenSecretReference(tokenSecretReferences()), imagePullSecretReferences()), createdDockercfgSecret()},
			DeletedSecret: serviceAccountTokenSecret(),

			ExpectedActions: []clientgotesting.Action{
				clientgotesting.NewListAction(schema.GroupVersionResource{Resource: "secrets"}, "default", metav1.ListOptions{FieldSelector: dockercfgSecretFieldSelector.String()}),
				clientgotesting.NewDeleteAction(schema.GroupVersionResource{Resource: "secrets"}, "default", "default-dockercfg-fplln"),
			},
		},
		"deleted token secret with serviceaccount with reference": {
			ClientObjects: []runtime.Object{serviceAccount(addTokenSecretReference(tokenSecretReferences()), imagePullSecretReferences()), createdDockercfgSecret()},

			DeletedSecret: serviceAccountTokenSecret(),
			ExpectedActions: []clientgotesting.Action{
				clientgotesting.NewListAction(schema.GroupVersionResource{Resource: "secrets"}, "default", metav1.ListOptions{FieldSelector: dockercfgSecretFieldSelector.String()}),
				clientgotesting.NewDeleteAction(schema.GroupVersionResource{Resource: "secrets"}, "default", "default-dockercfg-fplln"),
			},
		},
		"deleted token secret with serviceaccount without reference": {
			ClientObjects: []runtime.Object{serviceAccount(addTokenSecretReference(tokenSecretReferences()), imagePullSecretReferences()), createdDockercfgSecret()},

			DeletedSecret: serviceAccountTokenSecret(),
			ExpectedActions: []clientgotesting.Action{
				clientgotesting.NewListAction(schema.GroupVersionResource{Resource: "secrets"}, "default", metav1.ListOptions{FieldSelector: dockercfgSecretFieldSelector.String()}),
				clientgotesting.NewDeleteAction(schema.GroupVersionResource{Resource: "secrets"}, "default", "default-dockercfg-fplln"),
			},
		},
	}

	for k, tc := range testcases {
		// Re-seed to reset name generation
		rand.Seed(1)

		client := fake.NewSimpleClientset(tc.ClientObjects...)
		informerFactory := informers.NewSharedInformerFactory(client, controller.NoResyncPeriodFunc())
		controller := NewDockercfgTokenDeletedController(
			informerFactory.Core().InternalVersion().Secrets(),
			client,
			DockercfgTokenDeletedControllerOptions{},
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
