package controllers

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	openapi_v2 "github.com/googleapis/gnostic/OpenAPIv2"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/client-go/discovery"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"

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
		expectedActions            []string
		validateFunc               func(ts *testing.T, actionsKube []clientgotesting.Action, actionsDynamic []clientgotesting.Action, initialSecrets []*corev1.Secret, targetGRs []schema.GroupResource, unstructuredObjs []runtime.Object)
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
				func() runtime.Object {
					cm := createConfigMap("cm-1", "os")
					cm.Kind = "ConfigMap"
					cm.APIVersion = corev1.SchemeGroupVersion.String()
					return cm
				}(),
				func() runtime.Object {
					cm := createConfigMap("cm-2", "os")
					cm.Kind = "ConfigMap"
					cm.APIVersion = corev1.SchemeGroupVersion.String()
					return cm
				}(),
			},
			initialSecrets: nil,
			expectedActions: []string{
				"list:pods:kms",
				"get:secrets:kms",
				"list:secrets:openshift-config-managed",
			},
		},

		{
			name:            "a happy path scenario that tests resources encryption and secrets annotation",
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
				func() runtime.Object {
					cm := createConfigMap("cm-1", "os")
					cm.Kind = "ConfigMap"
					cm.APIVersion = corev1.SchemeGroupVersion.String()
					return cm
				}(),
				func() runtime.Object {
					cm := createConfigMap("cm-2", "os")
					cm.Kind = "ConfigMap"
					cm.APIVersion = corev1.SchemeGroupVersion.String()
					return cm
				}(),
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
				func() *corev1.Secret {
					s := &corev1.Secret{}
					s.Name = "s-in-abc"
					s.Namespace = "abc-ns"
					s.Kind = "Secret"
					s.APIVersion = corev1.SchemeGroupVersion.String()
					return s
				}(),
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
			validateFunc: func(ts *testing.T, actionsKube []clientgotesting.Action, actionsDynamic []clientgotesting.Action, initialSecrets []*corev1.Secret, targetGRs []schema.GroupResource, unstructuredObjs []runtime.Object) {
				// validate if the secrets were properly annotated
				validateSecretsWereAnnotated(ts, []schema.GroupResource{{Group: "", Resource: "configmaps"}, {Group: "", Resource: "secrets"}}, actionsKube, []*corev1.Secret{initialSecrets[0]})
				// validate if the resources were "encrypted"
				validateMigratedResources(ts, actionsDynamic, unstructuredObjs, targetGRs)
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
			scheme := runtime.NewScheme()
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
			fakeDynamicClient := dynamicfakeclient.NewSimpleDynamicClient(scheme, unstructuredObjs...)
			fakeDiscoveryClient := &fakeDisco{fakeKubeClient.Discovery(), []*metav1.APIResourceList{
				{
					TypeMeta:     metav1.TypeMeta{},
					APIResources: scenario.targetAPIResources,
				},
			}}

			deployer, err := encryptiondeployer.NewRevisionLabelPodDeployer("revision", scenario.targetNamespace, kubeInformers, nil, fakeKubeClient.CoreV1(), fakeSecretClient, encryptiondeployer.StaticPodNodeProvider{OperatorClient: fakeOperatorClient})
			if err != nil {
				t.Fatal(err)
			}

			// act
			target := NewMigrationController(
				deployer,
				fakeOperatorClient,
				kubeInformers,
				fakeSecretClient,
				scenario.encryptionSecretSelector,
				eventRecorder,
				scenario.targetGRs,
				fakeDynamicClient,
				fakeDiscoveryClient,
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
			if scenario.validateFunc != nil {
				scenario.validateFunc(t, fakeKubeClient.Actions(), fakeDynamicClient.Actions(), scenario.initialSecrets, scenario.targetGRs, unstructuredObjs)
			}
			if scenario.validateOperatorClientFunc != nil {
				scenario.validateOperatorClientFunc(t, fakeOperatorClient)
			}
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

func validateSecretsWereAnnotated(ts *testing.T, grs []schema.GroupResource, actions []clientgotesting.Action, expectedSecrets []*corev1.Secret) {
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
}

func createConfigMap(name, namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
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
