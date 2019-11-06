package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	operatorv1 "github.com/openshift/api/operator/v1"

	encryptiondeployer "github.com/openshift/library-go/pkg/operator/encryption/deployer"
	"github.com/openshift/library-go/pkg/operator/encryption/secrets"
	encryptiontesting "github.com/openshift/library-go/pkg/operator/encryption/testing"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

func TestMigrationController(t *testing.T) {
	scenarios := []struct {
		name                     string
		initialResources         []runtime.Object
		initialSecrets           []*corev1.Secret
		encryptionSecretSelector metav1.ListOptions
		targetNamespace          string
		targetGRs                []schema.GroupResource
		targetAPIResources       []metav1.APIResource
		// expectedActions holds actions to be verified in the form of "verb:resource:namespace"
		expectedActions []string

		expectedMigratorCalls []string
		migratorEnsureReplies map[schema.GroupResource]map[string]finishedResultErr
		migratorPruneReplies  map[schema.GroupResource]error

		validateFunc               func(ts *testing.T, actionsKube []clientgotesting.Action, initialSecrets []*corev1.Secret, targetGRs []schema.GroupResource, unstructuredObjs []runtime.Object)
		validateOperatorClientFunc func(ts *testing.T, operatorClient v1helpers.OperatorClient)
		expectedError              error
	}{
		{
			name:            "no config => nothing happens",
			targetNamespace: "kms",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
				{Group: "", Resource: "configmaps"},
			},
			targetAPIResources: []metav1.APIResource{
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
			},
			initialResources: []runtime.Object{
				encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1"),
			},
			initialSecrets: nil,
			expectedActions: []string{
				"list:pods:kms",
				"get:secrets:kms",
				"list:secrets:openshift-config-managed",
			},
		},

		{
			name:            "migrations are unfinished",
			targetNamespace: "kms",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
				{Group: "", Resource: "configmaps"},
			},
			targetAPIResources: []metav1.APIResource{
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
			},
			initialResources: []runtime.Object{
				encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1"),
			},
			initialSecrets: []*corev1.Secret{
				func() *corev1.Secret {
					s := encryptiontesting.CreateEncryptionKeySecretWithRawKey("kms", nil, 1, []byte("71ea7c91419a68fd1224f88d50316b4e"))
					s.Kind = "Secret"
					s.APIVersion = corev1.SchemeGroupVersion.String()
					return s
				}(),
				func() *corev1.Secret {
					keysResForSecrets := encryptiontesting.EncryptionKeysResourceTuple{
						Resource: "secrets",
						Keys: []apiserverconfigv1.Key{
							{
								Name:   "1",
								Secret: "NzFlYTdjOTE0MTlhNjhmZDEyMjRmODhkNTAzMTZiNGU=",
							},
						},
					}
					keysResForConfigMaps := encryptiontesting.EncryptionKeysResourceTuple{
						Resource: "configmaps",
						Keys: []apiserverconfigv1.Key{
							{
								Name:   "1",
								Secret: "NzFlYTdjOTE0MTlhNjhmZDEyMjRmODhkNTAzMTZiNGU=",
							},
						},
					}

					ec := encryptiontesting.CreateEncryptionCfgWithWriteKey([]encryptiontesting.EncryptionKeysResourceTuple{keysResForConfigMaps, keysResForSecrets})
					ecs := createEncryptionCfgSecret(t, "kms", "1", ec)
					ecs.APIVersion = corev1.SchemeGroupVersion.String()

					return ecs
				}(),
			},
			migratorEnsureReplies: map[schema.GroupResource]map[string]finishedResultErr{
				{Group: "", Resource: "secrets"}:    {"1": {finished: false}},
				{Group: "", Resource: "configmaps"}: {"1": {finished: false}},
			},
			expectedActions: []string{
				"list:pods:kms",
				"get:secrets:kms",
				"list:secrets:openshift-config-managed",
				"list:secrets:openshift-config-managed",
				"list:secrets:openshift-config-managed",
			},
			expectedMigratorCalls: []string{
				"ensure:configmaps:1",
				"ensure:secrets:1",
			},
			validateFunc: func(ts *testing.T, actionsKube []clientgotesting.Action, initialSecrets []*corev1.Secret, targetGRs []schema.GroupResource, unstructuredObjs []runtime.Object) {
				validateSecretsWereAnnotated(ts, []schema.GroupResource{}, actionsKube, nil, []*corev1.Secret{initialSecrets[0]})
			},
			validateOperatorClientFunc: func(ts *testing.T, operatorClient v1helpers.OperatorClient) {
				expectedConditions := []operatorv1.OperatorCondition{
					{
						Type:   "EncryptionMigrationControllerDegraded",
						Status: "False",
					},
					{
						Type:    "EncryptionMigrationControllerProgressing",
						Reason:  "Migrating",
						Message: "migrating resources to a new write key: [core/configmaps core/secrets]",
						Status:  "True",
					},
				}
				// TODO: test sequence of condition changes, not only the end result
				encryptiontesting.ValidateOperatorClientConditions(ts, operatorClient, expectedConditions)
			},
		},

		{
			name:            "configmaps are migrated, secrets are not finished",
			targetNamespace: "kms",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
				{Group: "", Resource: "configmaps"},
			},
			targetAPIResources: []metav1.APIResource{
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
			},
			initialResources: []runtime.Object{
				encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1"),
			},
			initialSecrets: []*corev1.Secret{
				func() *corev1.Secret {
					s := encryptiontesting.CreateEncryptionKeySecretWithRawKey("kms", nil, 1, []byte("71ea7c91419a68fd1224f88d50316b4e"))
					s.Kind = "Secret"
					s.APIVersion = corev1.SchemeGroupVersion.String()
					return s
				}(),
				func() *corev1.Secret {
					keysResForSecrets := encryptiontesting.EncryptionKeysResourceTuple{
						Resource: "secrets",
						Keys: []apiserverconfigv1.Key{
							{
								Name:   "1",
								Secret: "NzFlYTdjOTE0MTlhNjhmZDEyMjRmODhkNTAzMTZiNGU=",
							},
						},
					}
					keysResForConfigMaps := encryptiontesting.EncryptionKeysResourceTuple{
						Resource: "configmaps",
						Keys: []apiserverconfigv1.Key{
							{
								Name:   "1",
								Secret: "NzFlYTdjOTE0MTlhNjhmZDEyMjRmODhkNTAzMTZiNGU=",
							},
						},
					}

					ec := encryptiontesting.CreateEncryptionCfgWithWriteKey([]encryptiontesting.EncryptionKeysResourceTuple{keysResForConfigMaps, keysResForSecrets})
					ecs := createEncryptionCfgSecret(t, "kms", "1", ec)
					ecs.APIVersion = corev1.SchemeGroupVersion.String()

					return ecs
				}(),
			},
			migratorEnsureReplies: map[schema.GroupResource]map[string]finishedResultErr{
				{Group: "", Resource: "secrets"}:    {"1": {finished: false}},
				{Group: "", Resource: "configmaps"}: {"1": {finished: true}},
			},
			expectedActions: []string{
				"list:pods:kms",
				"get:secrets:kms",
				"list:secrets:openshift-config-managed",
				"list:secrets:openshift-config-managed",
				"get:secrets:openshift-config-managed",
				"get:secrets:openshift-config-managed",
				"update:secrets:openshift-config-managed",
				"create:events:operator",
				"list:secrets:openshift-config-managed",
			},
			expectedMigratorCalls: []string{
				"ensure:configmaps:1",
				"ensure:secrets:1",
			},
			validateFunc: func(ts *testing.T, actionsKube []clientgotesting.Action, initialSecrets []*corev1.Secret, targetGRs []schema.GroupResource, unstructuredObjs []runtime.Object) {
				validateSecretsWereAnnotated(ts, []schema.GroupResource{{Group: "", Resource: "configmaps"}}, actionsKube, []*corev1.Secret{initialSecrets[0]}, nil)
			},
			validateOperatorClientFunc: func(ts *testing.T, operatorClient v1helpers.OperatorClient) {
				expectedConditions := []operatorv1.OperatorCondition{
					{
						Type:   "EncryptionMigrationControllerDegraded",
						Status: "False",
					},
					{
						Type:    "EncryptionMigrationControllerProgressing",
						Reason:  "Migrating",
						Message: "migrating resources to a new write key: [core/secrets]",
						Status:  "True",
					},
				}
				// TODO: test sequence of condition changes, not only the end result
				encryptiontesting.ValidateOperatorClientConditions(ts, operatorClient, expectedConditions)
			},
		},

		{
			name:            "all migrations are finished",
			targetNamespace: "kms",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
				{Group: "", Resource: "configmaps"},
			},
			targetAPIResources: []metav1.APIResource{
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
			},
			initialResources: []runtime.Object{
				encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1"),
			},
			initialSecrets: []*corev1.Secret{
				func() *corev1.Secret {
					s := encryptiontesting.CreateEncryptionKeySecretWithRawKey("kms", nil, 1, []byte("71ea7c91419a68fd1224f88d50316b4e"))
					s.Kind = "Secret"
					s.APIVersion = corev1.SchemeGroupVersion.String()
					return s
				}(),
				func() *corev1.Secret {
					keysResForSecrets := encryptiontesting.EncryptionKeysResourceTuple{
						Resource: "secrets",
						Keys: []apiserverconfigv1.Key{
							{
								Name:   "1",
								Secret: "NzFlYTdjOTE0MTlhNjhmZDEyMjRmODhkNTAzMTZiNGU=",
							},
						},
					}
					keysResForConfigMaps := encryptiontesting.EncryptionKeysResourceTuple{
						Resource: "configmaps",
						Keys: []apiserverconfigv1.Key{
							{
								Name:   "1",
								Secret: "NzFlYTdjOTE0MTlhNjhmZDEyMjRmODhkNTAzMTZiNGU=",
							},
						},
					}

					ec := encryptiontesting.CreateEncryptionCfgWithWriteKey([]encryptiontesting.EncryptionKeysResourceTuple{keysResForConfigMaps, keysResForSecrets})
					ecs := createEncryptionCfgSecret(t, "kms", "1", ec)
					ecs.APIVersion = corev1.SchemeGroupVersion.String()

					return ecs
				}(),
			},
			migratorEnsureReplies: map[schema.GroupResource]map[string]finishedResultErr{
				{Group: "", Resource: "secrets"}:    {"1": {finished: true}},
				{Group: "", Resource: "configmaps"}: {"1": {finished: true}},
			},
			expectedActions: []string{
				"list:pods:kms",
				"get:secrets:kms",
				"list:secrets:openshift-config-managed",
				"list:secrets:openshift-config-managed",
				"get:secrets:openshift-config-managed",
				"get:secrets:openshift-config-managed",
				"update:secrets:openshift-config-managed",
				"create:events:operator",
				"list:secrets:openshift-config-managed",
				"get:secrets:openshift-config-managed",
				"get:secrets:openshift-config-managed",
				"update:secrets:openshift-config-managed",
				"create:events:operator",
			},
			expectedMigratorCalls: []string{
				"ensure:configmaps:1",
				"ensure:secrets:1",
			},
			validateFunc: func(ts *testing.T, actionsKube []clientgotesting.Action, initialSecrets []*corev1.Secret, targetGRs []schema.GroupResource, unstructuredObjs []runtime.Object) {
				validateSecretsWereAnnotated(ts, []schema.GroupResource{{Group: "", Resource: "configmaps"}, {Group: "", Resource: "secrets"}}, actionsKube, []*corev1.Secret{initialSecrets[0]}, nil)
			},
			validateOperatorClientFunc: func(ts *testing.T, operatorClient v1helpers.OperatorClient) {
				expectedConditions := []operatorv1.OperatorCondition{
					{
						Type:   "EncryptionMigrationControllerDegraded",
						Status: "False",
					},
					{
						Type:   "EncryptionMigrationControllerProgressing",
						Status: "False",
					},
				}
				// TODO: test sequence of condition changes, not only the end result
				encryptiontesting.ValidateOperatorClientConditions(ts, operatorClient, expectedConditions)
			},
		},

		{
			name:            "configmap migration failed",
			targetNamespace: "kms",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
				{Group: "", Resource: "configmaps"},
			},
			targetAPIResources: []metav1.APIResource{
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
			},
			initialResources: []runtime.Object{
				encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1"),
			},
			initialSecrets: []*corev1.Secret{
				func() *corev1.Secret {
					s := encryptiontesting.CreateEncryptionKeySecretWithRawKey("kms", nil, 1, []byte("71ea7c91419a68fd1224f88d50316b4e"))
					s.Kind = "Secret"
					s.APIVersion = corev1.SchemeGroupVersion.String()
					return s
				}(),
				func() *corev1.Secret {
					keysResForSecrets := encryptiontesting.EncryptionKeysResourceTuple{
						Resource: "secrets",
						Keys: []apiserverconfigv1.Key{
							{
								Name:   "1",
								Secret: "NzFlYTdjOTE0MTlhNjhmZDEyMjRmODhkNTAzMTZiNGU=",
							},
						},
					}
					keysResForConfigMaps := encryptiontesting.EncryptionKeysResourceTuple{
						Resource: "configmaps",
						Keys: []apiserverconfigv1.Key{
							{
								Name:   "1",
								Secret: "NzFlYTdjOTE0MTlhNjhmZDEyMjRmODhkNTAzMTZiNGU=",
							},
						},
					}

					ec := encryptiontesting.CreateEncryptionCfgWithWriteKey([]encryptiontesting.EncryptionKeysResourceTuple{keysResForConfigMaps, keysResForSecrets})
					ecs := createEncryptionCfgSecret(t, "kms", "1", ec)
					ecs.APIVersion = corev1.SchemeGroupVersion.String()

					return ecs
				}(),
			},
			migratorEnsureReplies: map[schema.GroupResource]map[string]finishedResultErr{
				{Group: "", Resource: "secrets"}:    {"1": {finished: false}},
				{Group: "", Resource: "configmaps"}: {"1": {finished: true, result: errors.New("configmap migration failed")}},
			},
			expectedError: errors.New("configmap migration failed"),
			expectedActions: []string{
				"list:pods:kms",
				"get:secrets:kms",
				"list:secrets:openshift-config-managed",
				"list:secrets:openshift-config-managed",
				"list:secrets:openshift-config-managed",
			},
			expectedMigratorCalls: []string{
				"ensure:configmaps:1",
				"ensure:secrets:1",
			},
			validateFunc: func(ts *testing.T, actionsKube []clientgotesting.Action, initialSecrets []*corev1.Secret, targetGRs []schema.GroupResource, unstructuredObjs []runtime.Object) {
				validateSecretsWereAnnotated(ts, []schema.GroupResource{}, actionsKube, nil, []*corev1.Secret{initialSecrets[0]})
			},
			validateOperatorClientFunc: func(ts *testing.T, operatorClient v1helpers.OperatorClient) {
				expectedConditions := []operatorv1.OperatorCondition{
					{
						Type:    "EncryptionMigrationControllerDegraded",
						Reason:  "Error",
						Message: "configmap migration failed",
						Status:  "True",
					},
					{
						Type:    "EncryptionMigrationControllerProgressing",
						Reason:  "Migrating",
						Message: "migrating resources to a new write key: [core/secrets]",
						Status:  "True",
					},
				}
				// TODO: test sequence of condition changes, not only the end result
				encryptiontesting.ValidateOperatorClientConditions(ts, operatorClient, expectedConditions)
			},
		},

		{
			name:            "configmap migration creation failed",
			targetNamespace: "kms",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
				{Group: "", Resource: "configmaps"},
			},
			targetAPIResources: []metav1.APIResource{
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
			},
			initialResources: []runtime.Object{
				encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1"),
			},
			initialSecrets: []*corev1.Secret{
				func() *corev1.Secret {
					s := encryptiontesting.CreateEncryptionKeySecretWithRawKey("kms", nil, 1, []byte("71ea7c91419a68fd1224f88d50316b4e"))
					s.Kind = "Secret"
					s.APIVersion = corev1.SchemeGroupVersion.String()
					return s
				}(),
				func() *corev1.Secret {
					keysResForSecrets := encryptiontesting.EncryptionKeysResourceTuple{
						Resource: "secrets",
						Keys: []apiserverconfigv1.Key{
							{
								Name:   "1",
								Secret: "NzFlYTdjOTE0MTlhNjhmZDEyMjRmODhkNTAzMTZiNGU=",
							},
						},
					}
					keysResForConfigMaps := encryptiontesting.EncryptionKeysResourceTuple{
						Resource: "configmaps",
						Keys: []apiserverconfigv1.Key{
							{
								Name:   "1",
								Secret: "NzFlYTdjOTE0MTlhNjhmZDEyMjRmODhkNTAzMTZiNGU=",
							},
						},
					}

					ec := encryptiontesting.CreateEncryptionCfgWithWriteKey([]encryptiontesting.EncryptionKeysResourceTuple{keysResForConfigMaps, keysResForSecrets})
					ecs := createEncryptionCfgSecret(t, "kms", "1", ec)
					ecs.APIVersion = corev1.SchemeGroupVersion.String()

					return ecs
				}(),
			},
			migratorEnsureReplies: map[schema.GroupResource]map[string]finishedResultErr{
				{Group: "", Resource: "secrets"}:    {"1": {finished: false}},
				{Group: "", Resource: "configmaps"}: {"1": {finished: false, err: errors.New("failed to start configmap migration")}},
			},
			expectedError: errors.New("failed to start configmap migration"),
			expectedActions: []string{
				"list:pods:kms",
				"get:secrets:kms",
				"list:secrets:openshift-config-managed",
				"list:secrets:openshift-config-managed",
				"list:secrets:openshift-config-managed",
			},
			expectedMigratorCalls: []string{
				"ensure:configmaps:1",
				"ensure:secrets:1",
			},
			validateFunc: func(ts *testing.T, actionsKube []clientgotesting.Action, initialSecrets []*corev1.Secret, targetGRs []schema.GroupResource, unstructuredObjs []runtime.Object) {
				validateSecretsWereAnnotated(ts, []schema.GroupResource{}, actionsKube, nil, []*corev1.Secret{initialSecrets[0]})
			},
			validateOperatorClientFunc: func(ts *testing.T, operatorClient v1helpers.OperatorClient) {
				expectedConditions := []operatorv1.OperatorCondition{
					{
						Type:    "EncryptionMigrationControllerDegraded",
						Reason:  "Error",
						Message: "failed to start configmap migration",
						Status:  "True",
					},
					{
						Type:    "EncryptionMigrationControllerProgressing",
						Reason:  "Migrating",
						Message: "migrating resources to a new write key: [core/secrets]",
						Status:  "True",
					},
				}
				// TODO: test sequence of condition changes, not only the end result
				encryptiontesting.ValidateOperatorClientConditions(ts, operatorClient, expectedConditions)
			},
		},

		// TODO: add more tests for not so happy paths
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
						Conditions: []operatorv1.OperatorCondition{
							{
								Type:   "EncryptionMigrationControllerDegraded",
								Status: "False",
							},
							{
								Type:   "EncryptionMigrationControllerProgressing",
								Status: operatorv1.ConditionFalse,
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

			allResources := []runtime.Object{}
			allResources = append(allResources, scenario.initialResources...)
			for _, initialSecret := range scenario.initialSecrets {
				allResources = append(allResources, initialSecret)
			}
			fakeKubeClient := fake.NewSimpleClientset(allResources...)
			eventRecorder := events.NewRecorder(fakeKubeClient.CoreV1().Events("operator"), "test-encryptionKeyController", &corev1.ObjectReference{})
			// we pass "openshift-config-managed" and $targetNamespace ns because the controller creates an informer for secrets in that namespace.
			// note that the informer factory is not used in the test - it's only needed to create the controller
			kubeInformers := v1helpers.NewKubeInformersForNamespaces(fakeKubeClient, "openshift-config-managed", scenario.targetNamespace)
			fakeSecretClient := fakeKubeClient.CoreV1()

			// let dynamic client know about the resources we want to encrypt
			resourceRequiresEncyrptionFunc := func(kind string) bool {
				if len(kind) == 0 {
					return false
				}
				for _, gr := range scenario.targetGRs {
					if strings.HasPrefix(gr.Resource, strings.ToLower(kind)) {
						return true
					}
				}
				return false
			}
			unstructuredObjs := []runtime.Object{}
			for _, rawObject := range allResources {
				rawUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rawObject.DeepCopyObject())
				if err != nil {
					t.Fatal(err)
				}
				unstructuredObj := &unstructured.Unstructured{Object: rawUnstructured}
				if resourceRequiresEncyrptionFunc(unstructuredObj.GetKind()) {
					unstructuredObjs = append(unstructuredObjs, unstructuredObj)
				}
			}

			deployer, err := encryptiondeployer.NewRevisionLabelPodDeployer("revision", scenario.targetNamespace, kubeInformers, nil, fakeKubeClient.CoreV1(), fakeSecretClient, encryptiondeployer.StaticPodNodeProvider{OperatorClient: fakeOperatorClient})
			if err != nil {
				t.Fatal(err)
			}
			migrator := &fakeMigrator{
				ensureReplies: scenario.migratorEnsureReplies,
				pruneReplies:  scenario.migratorPruneReplies,
			}

			// act
			target := NewMigrationController(
				deployer,
				migrator,
				fakeOperatorClient,
				kubeInformers,
				fakeSecretClient,
				scenario.encryptionSecretSelector,
				eventRecorder,
				scenario.targetGRs,
			)
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

			if err := encryptiontesting.ValidateActionsVerbs(fakeKubeClient.Actions(), scenario.expectedActions); err != nil {
				t.Fatalf("incorrect action(s) detected: %v", err)
			}
			if !reflect.DeepEqual(scenario.expectedMigratorCalls, migrator.calls) {
				t.Fatalf("incorrect migrator calls:\n  expected: %v\n       got: %v", scenario.expectedMigratorCalls, migrator.calls)
			}
			if scenario.validateFunc != nil {
				scenario.validateFunc(t, fakeKubeClient.Actions(), scenario.initialSecrets, scenario.targetGRs, unstructuredObjs)
			}
			if scenario.validateOperatorClientFunc != nil {
				scenario.validateOperatorClientFunc(t, fakeOperatorClient)
			}
		})
	}
}

func validateSecretsWereAnnotated(ts *testing.T, grs []schema.GroupResource, actions []clientgotesting.Action, expectedSecrets []*corev1.Secret, notExpectedSecrets []*corev1.Secret) {
	ts.Helper()

	lastSeen := map[string]*corev1.Secret{}
	for _, action := range actions {
		if !action.Matches("update", "secrets") {
			continue
		}
		updateAction := action.(clientgotesting.UpdateAction)
		actualSecret := updateAction.GetObject().(*corev1.Secret)
		lastSeen[fmt.Sprintf("%s/%s", actualSecret.Namespace, actualSecret.Name)] = actualSecret
	}

	for _, expected := range expectedSecrets {
		s, found := lastSeen[fmt.Sprintf("%s/%s", expected.Namespace, expected.Name)]
		if !found {
			ts.Errorf("missing update on %s/%s", expected.Namespace, expected.Name)
			continue
		}
		if _, ok := s.Annotations[secrets.EncryptionSecretMigratedTimestamp]; !ok {
			ts.Errorf("missing %s annotation on %s/%s", secrets.EncryptionSecretMigratedTimestamp, s.Namespace, s.Name)
		}
		if v, ok := s.Annotations[secrets.EncryptionSecretMigratedResources]; !ok {
			ts.Errorf("missing %s annotation on %s/%s", secrets.EncryptionSecretMigratedResources, s.Namespace, s.Name)
		} else {
			migratedGRs := secrets.MigratedGroupResources{}
			if err := json.Unmarshal([]byte(v), &migratedGRs); err != nil {
				ts.Errorf("failed to unmarshal %s annotation %q of secret %s/%s: %v", secrets.EncryptionSecretMigratedResources, v, s.Namespace, s.Name, err)
				continue
			}
			migratedGRsSet := map[string]bool{}
			for _, gr := range migratedGRs.Resources {
				migratedGRsSet[gr.String()] = true
			}
			for _, gr := range grs {
				if _, found := migratedGRsSet[gr.String()]; !found {
					ts.Errorf("missing resource %s in %s annotation on %s/%s", gr.String(), secrets.EncryptionSecretMigratedResources, s.Namespace, s.Name)
				}
			}
		}
	}

	for _, unexpected := range notExpectedSecrets {
		_, found := lastSeen[fmt.Sprintf("%s/%s", unexpected.Namespace, unexpected.Name)]
		if found {
			ts.Errorf("unexpected update on %s/%s", unexpected.Namespace, unexpected.Name)
			continue
		}
	}
}

type finishedResultErr struct {
	finished    bool
	result, err error
}

type fakeMigrator struct {
	calls         []string
	ensureReplies map[schema.GroupResource]map[string]finishedResultErr
	pruneReplies  map[schema.GroupResource]error
}

func (m *fakeMigrator) EnsureMigration(gr schema.GroupResource, writeKey string) (finished bool, result error, err error) {
	m.calls = append(m.calls, fmt.Sprintf("ensure:%s:%s", gr, writeKey))
	r := m.ensureReplies[gr][writeKey]
	return r.finished, r.result, r.err
}

func (m *fakeMigrator) PruneMigration(gr schema.GroupResource) error {
	m.calls = append(m.calls, fmt.Sprintf("prune:%s", gr))
	return m.pruneReplies[gr]
}

func (m *fakeMigrator) AddEventHandler(handler cache.ResourceEventHandler) []cache.InformerSynced {
	return nil
}
