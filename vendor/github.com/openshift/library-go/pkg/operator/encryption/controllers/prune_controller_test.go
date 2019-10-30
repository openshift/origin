package controllers

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	encryptiondeployer "github.com/openshift/library-go/pkg/operator/encryption/deployer"
	"github.com/openshift/library-go/pkg/operator/encryption/secrets"
	"github.com/openshift/library-go/pkg/operator/encryption/state"
	encryptiontesting "github.com/openshift/library-go/pkg/operator/encryption/testing"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

func TestPruneController(t *testing.T) {
	scenarios := []struct {
		name                     string
		initialSecrets           []*corev1.Secret
		encryptionSecretSelector metav1.ListOptions
		targetNamespace          string
		targetGRs                []schema.GroupResource
		// expectedActions holds actions to be verified in the form of "verb:resource:namespace"
		expectedActions       []string
		expectedEncryptionCfg *apiserverconfigv1.EncryptionConfiguration
		validateFunc          func(ts *testing.T, actions []clientgotesting.Action, initialSecrets []*corev1.Secret)
	}{
		{
			name:            "no-op only 10 keys were migrated",
			targetNamespace: "kms",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
			},
			initialSecrets: func() []*corev1.Secret {
				ns := "kms"
				all := []*corev1.Secret{}
				all = append(all, createMigratedEncryptionKeySecretsWithRndKey(t, 10, ns, "secrets")...)
				all = append(all, encryptiontesting.CreateEncryptionKeySecretWithRawKey(ns, nil, 11, []byte("cfbbae883984944e48d25590abdfd300")))
				return all
			}(),
			expectedActions: []string{"list:pods:kms", "get:secrets:kms", "list:secrets:openshift-config-managed", "list:secrets:openshift-config-managed"},
		},

		{
			name:            "14 keys were migrated, 1 of them is used, 10 are kept, the 3 most recent are pruned",
			targetNamespace: "kms",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
			},
			initialSecrets: createMigratedEncryptionKeySecretsWithRndKey(t, 14, "kms", "secrets"),
			expectedActions: []string{
				"list:pods:kms",
				"get:secrets:kms",
				"list:secrets:openshift-config-managed",
				"list:secrets:openshift-config-managed",
				"update:secrets:openshift-config-managed",
				"delete:secrets:openshift-config-managed",
				"update:secrets:openshift-config-managed",
				"delete:secrets:openshift-config-managed",
				"update:secrets:openshift-config-managed",
				"delete:secrets:openshift-config-managed",
			},
			validateFunc: func(ts *testing.T, actions []clientgotesting.Action, initialSecrets []*corev1.Secret) {
				validateSecretsWerePruned(ts, actions, initialSecrets[:3])
			},
		},

		{
			name:            "no-op the migrated keys don't match the selector",
			targetNamespace: "kms",
			targetGRs: []schema.GroupResource{
				{Group: "", Resource: "secrets"},
			},
			initialSecrets: func() []*corev1.Secret {
				return createMigratedEncryptionKeySecretsWithRndKey(t, 15, "not-kms", "secrets")
			}(),
			encryptionSecretSelector: metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", "encryption.apiserver.operator.openshift.io/component", "kms")},
			expectedActions: []string{
				"list:pods:kms",
				"get:secrets:kms",
				"list:secrets:openshift-config-managed",
				"list:secrets:openshift-config-managed",
			},
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
						Conditions: []operatorv1.OperatorCondition{
							{
								Type:   "EncryptionPruneControllerDegraded",
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

			rawSecrets := []runtime.Object{}
			for _, initialSecret := range scenario.initialSecrets {
				rawSecrets = append(rawSecrets, initialSecret)
			}

			fakePod := encryptiontesting.CreateDummyKubeAPIPod("kube-apiserver-1", "kms", "node-1")

			writeKeyRaw := []byte("71ea7c91419a68fd1224f88d50316b4e") // NzFlYTdjOTE0MTlhNjhmZDEyMjRmODhkNTAzMTZiNGU=
			writeKeyID := uint64(len(scenario.initialSecrets) + 1)
			writeKeySecret := encryptiontesting.CreateEncryptionKeySecretWithRawKey(scenario.targetNamespace, nil, writeKeyID, writeKeyRaw)

			initialKeys := []state.KeyState{}
			for _, s := range scenario.initialSecrets {
				km, err := secrets.ToKeyState(s)
				if err != nil {
					t.Fatal(err)
				}
				initialKeys = append(initialKeys, km)
			}

			encryptionConfig := func() *corev1.Secret {
				additionalReadKeys := state.KeysWithPotentiallyPersistedData(scenario.targetGRs, state.SortRecentFirst(initialKeys))
				var additionaConfigReadKeys []apiserverconfigv1.Key
				for _, rk := range additionalReadKeys {
					additionaConfigReadKeys = append(additionaConfigReadKeys, apiserverconfigv1.Key{
						Name:   rk.Key.Name,
						Secret: rk.Key.Secret,
					})
				}
				ec := encryptiontesting.CreateEncryptionCfgWithWriteKey([]encryptiontesting.EncryptionKeysResourceTuple{{
					Resource: "secrets",
					Keys: append([]apiserverconfigv1.Key{
						{
							Name:   fmt.Sprintf("%d", writeKeyID),
							Secret: base64.StdEncoding.EncodeToString(writeKeyRaw),
						},
					}, additionaConfigReadKeys...),
				}})
				ec.APIVersion = corev1.SchemeGroupVersion.String()
				return createEncryptionCfgSecret(t, "kms", "1", ec)
			}()
			fakeKubeClient := fake.NewSimpleClientset(append(rawSecrets, writeKeySecret, fakePod, encryptionConfig)...)
			eventRecorder := events.NewRecorder(fakeKubeClient.CoreV1().Events(scenario.targetNamespace), "test-encryptionKeyController", &corev1.ObjectReference{})
			// we pass "openshift-config-managed" and $targetNamespace ns because the controller creates an informer for secrets in that namespace.
			// note that the informer factory is not used in the test - it's only needed to create the controller
			kubeInformers := v1helpers.NewKubeInformersForNamespaces(fakeKubeClient, "openshift-config-managed", scenario.targetNamespace)
			fakeSecretClient := fakeKubeClient.CoreV1()

			deployer, err := encryptiondeployer.NewRevisionLabelPodDeployer("revision", scenario.targetNamespace, kubeInformers, nil, fakeKubeClient.CoreV1(), fakeSecretClient, encryptiondeployer.StaticPodNodeProvider{OperatorClient: fakeOperatorClient})
			if err != nil {
				t.Fatal(err)
			}

			target := NewPruneController(
				deployer,
				fakeOperatorClient,
				kubeInformers,
				fakeSecretClient,
				scenario.encryptionSecretSelector,
				eventRecorder,
				scenario.targetGRs,
			)

			// act
			err = target.sync()

			// validate
			if err != nil {
				t.Fatal(err)
			}
			if err := encryptiontesting.ValidateActionsVerbs(fakeKubeClient.Actions(), scenario.expectedActions); err != nil {
				t.Fatalf("incorrect action(s) detected: %v", err)
			}
			if scenario.validateFunc != nil {
				scenario.validateFunc(t, fakeKubeClient.Actions(), scenario.initialSecrets)
			}
		})
	}
}

func validateSecretsWerePruned(ts *testing.T, actions []clientgotesting.Action, expectedDeletedSecrets []*corev1.Secret) {
	ts.Helper()

	deletedSecretsCount := 0
	finalizersRemovedCount := 0
	for _, action := range actions {
		if action.GetVerb() == "update" {
			updateAction := action.(clientgotesting.UpdateAction)
			actualSecret := updateAction.GetObject().(*corev1.Secret)
			for _, expectedDeletedSecret := range expectedDeletedSecrets {
				if expectedDeletedSecret.Name == actualSecret.GetName() {
					expectedDeletedSecretsCpy := expectedDeletedSecret.DeepCopy()
					expectedDeletedSecretsCpy.Finalizers = []string{}
					if equality.Semantic.DeepEqual(actualSecret, expectedDeletedSecretsCpy) {
						finalizersRemovedCount++
						break
					}
				}
			}
		}
		if action.GetVerb() == "delete" {
			deleteAction := action.(clientgotesting.DeleteAction)
			for _, expectedDeletedSecret := range expectedDeletedSecrets {
				if expectedDeletedSecret.Name == deleteAction.GetName() && expectedDeletedSecret.Namespace == deleteAction.GetNamespace() {
					deletedSecretsCount++
				}
			}
		}
	}
	if deletedSecretsCount != len(expectedDeletedSecrets) {
		ts.Errorf("%d key(s) were deleted but %d were expected to be deleted", deletedSecretsCount, len(expectedDeletedSecrets))
	}
	if finalizersRemovedCount != len(expectedDeletedSecrets) {
		ts.Errorf("expected to see %d finalizers removed but got %d", len(expectedDeletedSecrets), finalizersRemovedCount)
	}
}

func createMigratedEncryptionKeySecretsWithRndKey(ts *testing.T, count int, namespace, resource string) []*corev1.Secret {
	ts.Helper()
	rawKey := make([]byte, 32)
	if _, err := rand.Read(rawKey); err != nil {
		ts.Fatal(err)
	}
	ret := []*corev1.Secret{}
	for i := 1; i <= count; i++ {
		s := encryptiontesting.CreateMigratedEncryptionKeySecretWithRawKey(namespace, []schema.GroupResource{{Group: "", Resource: resource}}, uint64(i), rawKey, time.Now())
		ret = append(ret, s)
	}
	return ret
}
