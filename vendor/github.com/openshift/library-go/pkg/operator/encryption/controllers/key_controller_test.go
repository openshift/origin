package controllers

import (
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configv1clientfake "github.com/openshift/client-go/config/clientset/versioned/fake"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions"

	encryptiondeployer "github.com/openshift/library-go/pkg/operator/encryption/deployer"
	encryptiontesting "github.com/openshift/library-go/pkg/operator/encryption/testing"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

func TestKeyController(t *testing.T) {
	apiServerAesCBC := []runtime.Object{&configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: configv1.APIServerSpec{
			Encryption: configv1.APIServerEncryption{
				Type: "aescbc",
			},
		},
	}}

	scenarios := []struct {
		name                     string
		initialObjects           []runtime.Object
		apiServerObjects         []runtime.Object
		encryptionSecretSelector metav1.ListOptions
		targetNamespace          string
		targetGRs                []schema.GroupResource
		// expectedActions holds actions to be verified in the form of "verb:resource:namespace"
		expectedActions            []string
		validateFunc               func(ts *testing.T, actions []clientgotesting.Action, targetNamespace string, targetGRs []schema.GroupResource)
		validateOperatorClientFunc func(ts *testing.T, operatorClient v1helpers.OperatorClient)
		expectedError              error
	}{
		{
			name: "no apiservers config",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
			},
			targetNamespace: "kms",
			initialObjects:  []runtime.Object{},
			validateFunc: func(ts *testing.T, actions []clientgotesting.Action, targetNamespace string, targetGRs []schema.GroupResource) {
			},
			expectedError:   fmt.Errorf(`apiservers.config.openshift.io "cluster" not found`),
			expectedActions: []string{},
		},

		{
			name: "no pod",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
			},
			targetNamespace:  "kms",
			initialObjects:   []runtime.Object{},
			apiServerObjects: []runtime.Object{&configv1.APIServer{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}}},
			validateFunc: func(ts *testing.T, actions []clientgotesting.Action, targetNamespace string, targetGRs []schema.GroupResource) {
			},
			expectedActions: []string{"list:pods:kms"},
		},

		{
			name: "encryption disabled",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
			},
			targetNamespace:  "kms",
			initialObjects:   []runtime.Object{encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1")},
			apiServerObjects: []runtime.Object{&configv1.APIServer{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}}},
			validateFunc: func(ts *testing.T, actions []clientgotesting.Action, targetNamespace string, targetGRs []schema.GroupResource) {
			},
			expectedActions: []string{"list:pods:kms", "get:secrets:kms", "list:secrets:openshift-config-managed"},
		},

		// Assumes a clean slate, that is, there are no previous resources in the system.
		// It expects that a secret resource with an appropriate key, name and labels will be created.
		{
			name: "checks if a secret with AES256 key for core/secret is created",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
			},
			targetNamespace: "kms",
			expectedActions: []string{"list:pods:kms", "get:secrets:kms", "list:secrets:openshift-config-managed", "create:secrets:openshift-config-managed", "create:events:kms"},
			initialObjects: []runtime.Object{
				encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1"),
			},
			apiServerObjects: []runtime.Object{&configv1.APIServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: configv1.APIServerSpec{
					Encryption: configv1.APIServerEncryption{
						Type: "aescbc",
					},
				},
			}},
			validateFunc: func(ts *testing.T, actions []clientgotesting.Action, targetNamespace string, targetGRs []schema.GroupResource) {
				wasSecretValidated := false
				for _, action := range actions {
					if action.Matches("create", "secrets") {
						createAction := action.(clientgotesting.CreateAction)
						actualSecret := createAction.GetObject().(*corev1.Secret)
						expectedSecret := encryptiontesting.CreateEncryptionKeySecretWithKeyFromExistingSecret(targetNamespace, []schema.GroupResource{}, 1, actualSecret)
						expectedSecret.Annotations["encryption.apiserver.operator.openshift.io/internal-reason"] = "secrets-key-does-not-exist" // TODO: Fix this
						if !equality.Semantic.DeepEqual(actualSecret, expectedSecret) {
							ts.Errorf(diff.ObjectDiff(expectedSecret, actualSecret))
						}
						if err := encryptiontesting.ValidateEncryptionKey(actualSecret); err != nil {
							ts.Error(err)
						}
						wasSecretValidated = true
						break
					}
				}
				if !wasSecretValidated {
					ts.Errorf("the secret wasn't created and validated")
				}
			},
		},

		{
			name: "no-op when a valid write key exists, but is not migrated",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
			},
			initialObjects: []runtime.Object{
				encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1"),
				encryptiontesting.CreateEncryptionKeySecretWithRawKey("kms", nil, 7, []byte("61def964fb967f5d7c44a2af8dab6865")),
			},
			apiServerObjects: apiServerAesCBC,
			targetNamespace:  "kms",
			expectedActions:  []string{"list:pods:kms", "get:secrets:kms", "list:secrets:openshift-config-managed"},
		},

		{
			name: "no-op when a valid write key exists, is migrated, but not expired",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
			},
			initialObjects: []runtime.Object{
				encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1"),
				encryptiontesting.CreateMigratedEncryptionKeySecretWithRawKey("kms", []schema.GroupResource{{Group: "", Resource: "secrets"}}, 7, []byte("61def964fb967f5d7c44a2af8dab6865"), time.Now()),
			},
			apiServerObjects: apiServerAesCBC,
			targetNamespace:  "kms",
			expectedActions:  []string{"list:pods:kms", "get:secrets:kms", "list:secrets:openshift-config-managed"},
		},

		{
			name: "creates a new write key because previous one is migrated, but has no migration timestamp",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
			},
			initialObjects: []runtime.Object{
				encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1"),
				encryptiontesting.CreateEncryptionKeySecretWithRawKey("kms", []schema.GroupResource{{Group: "", Resource: "secrets"}}, 7, []byte("61def964fb967f5d7c44a2af8dab6865")),
			},
			apiServerObjects: apiServerAesCBC,
			targetNamespace:  "kms",
			expectedActions:  []string{"list:pods:kms", "get:secrets:kms", "list:secrets:openshift-config-managed", "create:secrets:openshift-config-managed", "create:events:kms"},
		},

		{
			name: "creates a new write key because the previous one expired",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
			},
			initialObjects: []runtime.Object{
				encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1"),
				encryptiontesting.CreateExpiredMigratedEncryptionKeySecretWithRawKey("kms", []schema.GroupResource{{Group: "", Resource: "secrets"}}, 5, []byte("61def964fb967f5d7c44a2af8dab6865")),
			},
			apiServerObjects: apiServerAesCBC,
			targetNamespace:  "kms",
			expectedActions:  []string{"list:pods:kms", "get:secrets:kms", "list:secrets:openshift-config-managed", "create:secrets:openshift-config-managed", "create:events:kms"},
			validateFunc: func(ts *testing.T, actions []clientgotesting.Action, targetNamespace string, targetGRs []schema.GroupResource) {
				wasSecretValidated := false
				for _, action := range actions {
					if action.Matches("create", "secrets") {
						createAction := action.(clientgotesting.CreateAction)
						actualSecret := createAction.GetObject().(*corev1.Secret)
						expectedSecret := encryptiontesting.CreateEncryptionKeySecretWithKeyFromExistingSecret(targetNamespace, []schema.GroupResource{}, 6, actualSecret)
						expectedSecret.Annotations["encryption.apiserver.operator.openshift.io/internal-reason"] = "secrets-rotation-interval-has-passed"
						if !equality.Semantic.DeepEqual(actualSecret, expectedSecret) {
							ts.Errorf(diff.ObjectDiff(expectedSecret, actualSecret))
						}
						if err := encryptiontesting.ValidateEncryptionKey(actualSecret); err != nil {
							ts.Error(err)
						}
						wasSecretValidated = true
						break
					}
				}
				if !wasSecretValidated {
					ts.Errorf("the secret wasn't created and validated")
				}
			},
		},

		{
			name: "create a new write key when the previous key expired and another read key exists",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
			},
			initialObjects: []runtime.Object{
				encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1"),
				encryptiontesting.CreateExpiredMigratedEncryptionKeySecretWithRawKey("kms", []schema.GroupResource{{Group: "", Resource: "secrets"}}, 6, []byte("61def964fb967f5d7c44a2af8dab6865")),
				encryptiontesting.CreateEncryptionKeySecretWithRawKey("kms", nil, 5, []byte("51def964fb967f5d7c44a2af8dab6865")),
				func() *corev1.Secret {
					keysResForSecrets := encryptiontesting.EncryptionKeysResourceTuple{
						Resource: "secrets",
						Keys: []apiserverconfigv1.Key{
							{
								Name:   "6",
								Secret: base64.StdEncoding.EncodeToString([]byte("61def964fb967f5d7c44a2af8dab6865")),
							},
							{
								Name:   "5",
								Secret: base64.StdEncoding.EncodeToString([]byte("51def964fb967f5d7c44a2af8dab6865")),
							},
						},
					}

					ec := encryptiontesting.CreateEncryptionCfgWithWriteKey([]encryptiontesting.EncryptionKeysResourceTuple{keysResForSecrets})
					ecs := createEncryptionCfgSecret(t, "kms", "1", ec)
					ecs.APIVersion = corev1.SchemeGroupVersion.String()

					return ecs
				}(),
			},
			apiServerObjects: apiServerAesCBC,
			targetNamespace:  "kms",
			expectedActions: []string{
				"list:pods:kms",
				"get:secrets:kms",
				"list:secrets:openshift-config-managed",
				"create:secrets:openshift-config-managed",
				"create:events:kms",
			},
		},

		{
			name: "no-op when the previous key was migrated and the current one is valid but hasn't been observed",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
			},
			initialObjects: []runtime.Object{
				encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1"),
				encryptiontesting.CreateExpiredMigratedEncryptionKeySecretWithRawKey("kms", []schema.GroupResource{{Group: "", Resource: "secrets"}}, 5, []byte("61def964fb967f5d7c44a2af8dab6865")),
				encryptiontesting.CreateEncryptionKeySecretWithRawKey("kms", nil, 6, []byte("61def964fb967f5d7c44a2af8dab6865")),
			},
			apiServerObjects: apiServerAesCBC,
			targetNamespace:  "kms",
			expectedActions:  []string{"list:pods:kms", "get:secrets:kms", "list:secrets:openshift-config-managed"},
		},

		{
			name: "degraded a secret with invalid key exists",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
			},
			initialObjects: []runtime.Object{
				encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1"),
				encryptiontesting.CreateEncryptionKeySecretWithRawKey("kms", nil, 1, []byte("")),
			},
			apiServerObjects: apiServerAesCBC,
			targetNamespace:  "kms",
			expectedActions:  []string{"list:pods:kms", "get:secrets:kms", "list:secrets:openshift-config-managed", "create:secrets:openshift-config-managed", "get:secrets:openshift-config-managed"},
			validateOperatorClientFunc: func(ts *testing.T, operatorClient v1helpers.OperatorClient) {
				expectedCondition := operatorv1.OperatorCondition{
					Type:    "EncryptionKeyControllerDegraded",
					Status:  "True",
					Reason:  "Error",
					Message: "secret encryption-key-kms-1 is invalid, new keys cannot be created for encryption target",
				}
				encryptiontesting.ValidateOperatorClientConditions(ts, operatorClient, []operatorv1.OperatorCondition{expectedCondition})
			},
			expectedError: errors.New("secret encryption-key-kms-1 is invalid, new keys cannot be created for encryption target"),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// setup
			fakeOperatorClient := v1helpers.NewFakeStaticPodOperatorClient(
				&operatorv1.StaticPodOperatorSpec{
					OperatorSpec: operatorv1.OperatorSpec{
						ManagementState: operatorv1.Managed,
					},
				},
				&operatorv1.StaticPodOperatorStatus{
					OperatorStatus: operatorv1.OperatorStatus{
						// we need to set up proper conditions before the test starts because
						// the controller calls UpdateStatus which calls UpdateOperatorStatus method which is unsupported (fake client) and throws an exception
						Conditions: []operatorv1.OperatorCondition{
							{
								Type:   "EncryptionKeyControllerDegraded",
								Status: "False",
							},
						},
					},
					NodeStatuses: []operatorv1.NodeStatus{
						{NodeName: "node-1"},
					},
				},
				nil,
				nil,
			)

			fakeKubeClient := fake.NewSimpleClientset(scenario.initialObjects...)
			eventRecorder := events.NewRecorder(fakeKubeClient.CoreV1().Events(scenario.targetNamespace), "test-encryptionKeyController", &corev1.ObjectReference{})
			// pass informer for
			// - target namespace: pods and secrets
			// - openshift-config-managed: secrets
			// note that the informer factory is not used in the test - it's only needed to create the controller
			kubeInformers := v1helpers.NewKubeInformersForNamespaces(fakeKubeClient, "openshift-config-managed", scenario.targetNamespace)
			fakeSecretClient := fakeKubeClient.CoreV1()
			fakePodClient := fakeKubeClient.CoreV1()
			fakeConfigClient := configv1clientfake.NewSimpleClientset(scenario.apiServerObjects...)
			fakeApiServerClient := fakeConfigClient.ConfigV1().APIServers()
			fakeApiServerInformer := configv1informers.NewSharedInformerFactory(fakeConfigClient, time.Minute).Config().V1().APIServers()

			deployer, err := encryptiondeployer.NewRevisionLabelPodDeployer("revision", scenario.targetNamespace, kubeInformers, nil, fakePodClient, fakeSecretClient, encryptiondeployer.StaticPodNodeProvider{OperatorClient: fakeOperatorClient})
			if err != nil {
				t.Fatal(err)
			}

			target := NewKeyController(scenario.targetNamespace, deployer, fakeOperatorClient, fakeApiServerClient, fakeApiServerInformer, kubeInformers, fakeSecretClient, scenario.encryptionSecretSelector, eventRecorder, scenario.targetGRs)

			// act
			err = target.sync()

			// validate
			if err == nil && scenario.expectedError != nil {
				t.Fatal("expected to get an error from sync() method")
			}
			if err != nil && scenario.expectedError == nil {
				t.Fatal(err)
			}
			if err != nil && scenario.expectedError != nil && err.Error() != scenario.expectedError.Error() {
				t.Fatalf("unexpected error returned = %v, expected = %v", err, scenario.expectedError)
			}
			if err := encryptiontesting.ValidateActionsVerbs(fakeKubeClient.Actions(), scenario.expectedActions); err != nil {
				t.Fatalf("incorrect action(s) detected: %v", err)
			}
			if scenario.validateFunc != nil {
				scenario.validateFunc(t, fakeKubeClient.Actions(), scenario.targetNamespace, scenario.targetGRs)
			}
			if scenario.validateOperatorClientFunc != nil {
				scenario.validateOperatorClientFunc(t, fakeOperatorClient)
			}
		})
	}
}
