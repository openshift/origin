package controllers

import (
	"math/rand"
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	informers "k8s.io/client-go/informers"
	externalfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/controller"
)

// emptySecretReferences is used by a service account without any secrets
func emptySecretReferences() []v1.ObjectReference {
	return []v1.ObjectReference{}
}

func emptyImagePullSecretReferences() []v1.LocalObjectReference {
	return []v1.LocalObjectReference{}
}

// missingSecretReferences is used by a service account that references secrets which do no exist
func missingSecretReferences() []v1.ObjectReference {
	return []v1.ObjectReference{{Name: "missing-secret-1"}}
}

// regularSecretReferences is used by a service account that references secrets which are not ServiceAccountTokens
func regularSecretReferences() []v1.ObjectReference {
	return []v1.ObjectReference{{Name: "regular-secret-1"}}
}

// tokenSecretReferences is used by a service account that references a ServiceAccountToken secret
func tokenSecretReferences() []v1.ObjectReference {
	return []v1.ObjectReference{{Name: "token-secret-1"}}
}

// addTokenSecretReference adds a reference to the ServiceAccountToken that will be created
func addTokenSecretReference(refs []v1.ObjectReference) []v1.ObjectReference {
	return append(refs, v1.ObjectReference{Name: "default-dockercfg-fplln"})
}

func imagePullSecretReferences() []v1.LocalObjectReference {
	return []v1.LocalObjectReference{{Name: "default-dockercfg-fplln"}}
}

// serviceAccount returns a service account with the given secret refs
func serviceAccount(secretRefs []v1.ObjectReference, imagePullSecretRefs []v1.LocalObjectReference) *v1.ServiceAccount {
	return &v1.ServiceAccount{
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
func createdDockercfgSecret() *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-dockercfg-fplln",
			Namespace: "default",
			Annotations: map[string]string{
				v1.ServiceAccountNameKey:         "default",
				v1.ServiceAccountUIDKey:          "12345",
				ServiceAccountTokenSecretNameKey: "token-secret-1",
			},
		},
		Type: v1.SecretTypeDockercfg,
		Data: map[string][]byte{
			v1.DockerConfigKey: []byte(`{"docker-registry.default.svc.cluster.local":{"Username":"serviceaccount","Password":"ABC","Email":"serviceaccount@example.org"}}`),
		},
	}
}

// opaqueSecret returns a persisted non-ServiceAccountToken secret named "regular-secret-1"
func opaqueSecret() *v1.Secret {
	return &v1.Secret{
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
func serviceAccountTokenSecret() *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "token-secret-1",
			Namespace:       "default",
			UID:             "23456",
			ResourceVersion: "1",
			Annotations: map[string]string{
				v1.ServiceAccountNameKey: "default",
				v1.ServiceAccountUIDKey:  "12345",
			},
		},
		Type: v1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{
			"token": []byte("ABC"),
		},
	}
}

// serviceAccountTokenSecretWithoutTokenData returns an existing ServiceAccountToken secret that lacks token data
func serviceAccountTokenSecretWithoutTokenData() *v1.Secret {
	secret := serviceAccountTokenSecret()
	secret.Data = nil
	return secret
}

func TestTokenDeletion(t *testing.T) {
	dockercfgSecretFieldSelector := fields.OneTermEqualSelector(api.SecretTypeField, string(v1.SecretTypeDockercfg))

	testcases := map[string]struct {
		ClientObjects []runtime.Object

		DeletedSecret *v1.Secret

		ExpectedActions []clientgotesting.Action
	}{
		"deleted token secret without serviceaccount": {
			ClientObjects: []runtime.Object{serviceAccount(addTokenSecretReference(tokenSecretReferences()), imagePullSecretReferences()), createdDockercfgSecret()},
			DeletedSecret: serviceAccountTokenSecret(),

			ExpectedActions: []clientgotesting.Action{
				clientgotesting.NewListAction(
					schema.GroupVersionResource{Resource: "secrets", Version: "v1"},
					schema.GroupVersionKind{Kind: "Secret", Version: "v1"},
					"default", metav1.ListOptions{FieldSelector: dockercfgSecretFieldSelector.String()}),
				clientgotesting.NewDeleteAction(schema.GroupVersionResource{Resource: "secrets", Version: "v1"}, "default", "default-dockercfg-fplln"),
			},
		},
		"deleted token secret with serviceaccount with reference": {
			ClientObjects: []runtime.Object{serviceAccount(addTokenSecretReference(tokenSecretReferences()), imagePullSecretReferences()), createdDockercfgSecret()},

			DeletedSecret: serviceAccountTokenSecret(),
			ExpectedActions: []clientgotesting.Action{
				clientgotesting.NewListAction(
					schema.GroupVersionResource{Resource: "secrets", Version: "v1"},
					schema.GroupVersionKind{Kind: "Secret", Version: "v1"},
					"default", metav1.ListOptions{FieldSelector: dockercfgSecretFieldSelector.String()}),
				clientgotesting.NewDeleteAction(schema.GroupVersionResource{Resource: "secrets", Version: "v1"}, "default", "default-dockercfg-fplln"),
			},
		},
		"deleted token secret with serviceaccount without reference": {
			ClientObjects: []runtime.Object{serviceAccount(addTokenSecretReference(tokenSecretReferences()), imagePullSecretReferences()), createdDockercfgSecret()},

			DeletedSecret: serviceAccountTokenSecret(),
			ExpectedActions: []clientgotesting.Action{
				clientgotesting.NewListAction(
					schema.GroupVersionResource{Resource: "secrets", Version: "v1"},
					schema.GroupVersionKind{Kind: "Secret", Version: "v1"},
					"default", metav1.ListOptions{FieldSelector: dockercfgSecretFieldSelector.String()}),
				clientgotesting.NewDeleteAction(schema.GroupVersionResource{Resource: "secrets", Version: "v1"}, "default", "default-dockercfg-fplln"),
			},
		},
	}

	for k, tc := range testcases {
		// Re-seed to reset name generation
		rand.Seed(1)

		client := externalfake.NewSimpleClientset(tc.ClientObjects...)
		informerFactory := informers.NewSharedInformerFactory(client, controller.NoResyncPeriodFunc())
		controller := NewDockercfgTokenDeletedController(
			informerFactory.Core().V1().Secrets(),
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
