package revision

import (
	"strings"
	"testing"
	"time"

	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
)

func filterCreateActions(actions []clienttesting.Action) []runtime.Object {
	var createdObjects []runtime.Object
	for _, a := range actions {
		createAction, isCreate := a.(clienttesting.CreateAction)
		if !isCreate {
			continue
		}
		_, isEvent := createAction.GetObject().(*v1.Event)
		if isEvent {
			continue
		}
		createdObjects = append(createdObjects, createAction.GetObject())
	}
	return createdObjects
}

const targetNamespace = "copy-resources"

func TestRevisionController(t *testing.T) {
	tests := []struct {
		testName                string
		targetNamespace         string
		testSecrets             []RevisionResource
		testConfigs             []RevisionResource
		startingObjects         []runtime.Object
		staticPodOperatorClient v1helpers.StaticPodOperatorClient
		validateActions         func(t *testing.T, actions []clienttesting.Action)
		validateStatus          func(t *testing.T, status *operatorv1.StaticPodOperatorStatus)
		expectSyncError         string
	}{
		{
			testName:        "operator-unmanaged",
			targetNamespace: targetNamespace,
			staticPodOperatorClient: v1helpers.NewFakeStaticPodOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Unmanaged,
				},
				&operatorv1.OperatorStatus{},
				&operatorv1.StaticPodOperatorSpec{
					OperatorSpec: operatorv1.OperatorSpec{
						ManagementState: operatorv1.Unmanaged,
					},
				},
				&operatorv1.StaticPodOperatorStatus{},
				nil,
			),
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				createdObjects := filterCreateActions(actions)
				if createdObjectCount := len(createdObjects); createdObjectCount != 0 {
					t.Errorf("expected no objects to be created, got %d", createdObjectCount)
				}
			},
		},
		{
			testName:        "missing-source-resources",
			targetNamespace: targetNamespace,
			staticPodOperatorClient: v1helpers.NewFakeStaticPodOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
				&operatorv1.OperatorStatus{},
				&operatorv1.StaticPodOperatorSpec{
					OperatorSpec: operatorv1.OperatorSpec{
						ManagementState: operatorv1.Managed,
					},
				},
				&operatorv1.StaticPodOperatorStatus{
					LatestAvailableRevision: 1,
					NodeStatuses: []operatorv1.NodeStatus{
						{
							NodeName:        "test-node-1",
							CurrentRevision: 0,
							TargetRevision:  0,
						},
					},
				},
				nil,
			),
			testConfigs:     []RevisionResource{{Name: "test-config"}},
			testSecrets:     []RevisionResource{{Name: "test-secret"}},
			expectSyncError: "synthetic requeue request",
			validateStatus: func(t *testing.T, status *operatorv1.StaticPodOperatorStatus) {
				if status.Conditions[0].Type != "RevisionControllerFailing" {
					t.Errorf("expected status condition to be 'RevisionControllerFailing', got %v", status.Conditions[0].Type)
				}
				if status.Conditions[0].Reason != "ContentCreationError" {
					t.Errorf("expected status condition reason to be 'ContentCreationError', got %v", status.Conditions[0].Reason)
				}
				if !strings.Contains(status.Conditions[0].Message, `configmaps "test-config" not found`) {
					t.Errorf("expected status to be 'configmaps test-config not found', got: %s", status.Conditions[0].Message)
				}
			},
		},
		{
			testName:        "copy-resources",
			targetNamespace: targetNamespace,
			staticPodOperatorClient: v1helpers.NewFakeStaticPodOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
				&operatorv1.OperatorStatus{},
				&operatorv1.StaticPodOperatorSpec{
					OperatorSpec: operatorv1.OperatorSpec{
						ManagementState: operatorv1.Managed,
					},
				},
				&operatorv1.StaticPodOperatorStatus{
					LatestAvailableRevision: 0,
					NodeStatuses: []operatorv1.NodeStatus{
						{
							NodeName:        "test-node-1",
							CurrentRevision: 0,
							TargetRevision:  0,
						},
					},
				},
				nil,
			),
			startingObjects: []runtime.Object{
				&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test-secret", Namespace: targetNamespace}},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: targetNamespace}},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status", Namespace: targetNamespace}},
			},
			testConfigs: []RevisionResource{{Name: "test-config"}},
			testSecrets: []RevisionResource{{Name: "test-secret"}},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				createdObjects := filterCreateActions(actions)
				if createdObjectCount := len(createdObjects); createdObjectCount != 3 {
					t.Errorf("expected 3 objects to be created, got %d: %+v", createdObjectCount, createdObjects)
					return
				}
				revisionStatus, hasStatus := createdObjects[0].(*v1.ConfigMap)
				if !hasStatus {
					t.Errorf("expected config to be created")
					return
				}
				if revisionStatus.Name != "revision-status-1" {
					t.Errorf("expected config to have name 'revision-status-1', got %q", revisionStatus.Name)
				}
				config, hasConfig := createdObjects[1].(*v1.ConfigMap)
				if !hasConfig {
					t.Errorf("expected config to be created")
					return
				}
				if config.Name != "test-config-1" {
					t.Errorf("expected config to have name 'test-config-1', got %q", config.Name)
				}
				if len(config.OwnerReferences) != 1 {
					t.Errorf("expected config to have ownerreferences set, got %+v", config.OwnerReferences)
				}
				secret, hasSecret := createdObjects[2].(*v1.Secret)
				if !hasSecret {
					t.Errorf("expected secret to be created")
					return
				}
				if secret.Name != "test-secret-1" {
					t.Errorf("expected secret to have name 'test-secret-1', got %q", secret.Name)
				}
				if len(secret.OwnerReferences) != 1 {
					t.Errorf("expected secret to have ownerreferences set, got %+v", secret.OwnerReferences)
				}
			},
		},
		{
			testName:        "copy-resources-opt",
			targetNamespace: targetNamespace,
			staticPodOperatorClient: v1helpers.NewFakeStaticPodOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
				&operatorv1.OperatorStatus{},
				&operatorv1.StaticPodOperatorSpec{
					OperatorSpec: operatorv1.OperatorSpec{
						ManagementState: operatorv1.Managed,
					},
				},
				&operatorv1.StaticPodOperatorStatus{
					LatestAvailableRevision: 0,
					NodeStatuses: []operatorv1.NodeStatus{
						{
							NodeName:        "test-node-1",
							CurrentRevision: 0,
							TargetRevision:  0,
						},
					},
				},
				nil,
			),
			startingObjects: []runtime.Object{
				&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test-secret", Namespace: targetNamespace}},
				&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test-secret-opt", Namespace: targetNamespace}},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: targetNamespace}},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-config-opt", Namespace: targetNamespace}},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status", Namespace: targetNamespace}},
			},
			testConfigs: []RevisionResource{{Name: "test-config"}, {Name: "test-config-opt", Optional: true}},
			testSecrets: []RevisionResource{{Name: "test-secret"}, {Name: "test-secret-opt", Optional: true}},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				createdObjects := filterCreateActions(actions)
				if createdObjectCount := len(createdObjects); createdObjectCount != 5 {
					t.Errorf("expected 5 objects to be created, got %d: %+v", createdObjectCount, createdObjects)
					return
				}
				revisionStatus, hasStatus := createdObjects[0].(*v1.ConfigMap)
				if !hasStatus {
					t.Errorf("expected config to be created")
					return
				}
				if revisionStatus.Name != "revision-status-1" {
					t.Errorf("expected config to have name 'revision-status-1', got %q", revisionStatus.Name)
				}
				config, hasConfig := createdObjects[1].(*v1.ConfigMap)
				if !hasConfig {
					t.Errorf("expected config to be created")
					return
				}
				if config.Name != "test-config-1" {
					t.Errorf("expected config to have name 'test-config-1', got %q", config.Name)
				}
				config, hasConfig = createdObjects[2].(*v1.ConfigMap)
				if !hasConfig {
					t.Errorf("expected config to be created")
					return
				}
				if config.Name != "test-config-opt-1" {
					t.Errorf("expected config to have name 'test-config-opt-1', got %q", config.Name)
				}
				secret, hasSecret := createdObjects[3].(*v1.Secret)
				if !hasSecret {
					t.Errorf("expected secret to be created")
					return
				}
				if secret.Name != "test-secret-1" {
					t.Errorf("expected secret to have name 'test-secret-1', got %q", secret.Name)
				}
				secret, hasSecret = createdObjects[4].(*v1.Secret)
				if !hasSecret {
					t.Errorf("expected secret to be created")
					return
				}
				if secret.Name != "test-secret-opt-1" {
					t.Errorf("expected secret to have name 'test-secret-opt-1', got %q", secret.Name)
				}
			},
		},
		{
			testName:        "copy-resources-opt-missing",
			targetNamespace: targetNamespace,
			staticPodOperatorClient: v1helpers.NewFakeStaticPodOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
				&operatorv1.OperatorStatus{},
				&operatorv1.StaticPodOperatorSpec{
					OperatorSpec: operatorv1.OperatorSpec{
						ManagementState: operatorv1.Managed,
					},
				},
				&operatorv1.StaticPodOperatorStatus{
					LatestAvailableRevision: 0,
					NodeStatuses: []operatorv1.NodeStatus{
						{
							NodeName:        "test-node-1",
							CurrentRevision: 0,
							TargetRevision:  0,
						},
					},
				},
				nil,
			),
			startingObjects: []runtime.Object{
				&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test-secret", Namespace: targetNamespace}},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: targetNamespace}},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status", Namespace: targetNamespace}},
			},
			testConfigs: []RevisionResource{{Name: "test-config"}, {Name: "test-config-opt", Optional: true}},
			testSecrets: []RevisionResource{{Name: "test-secret"}, {Name: "test-secret-opt", Optional: true}},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				createdObjects := filterCreateActions(actions)
				if createdObjectCount := len(createdObjects); createdObjectCount != 3 {
					t.Errorf("expected 3 objects to be created, got %d: %+v", createdObjectCount, createdObjects)
					return
				}
				revisionStatus, hasStatus := createdObjects[0].(*v1.ConfigMap)
				if !hasStatus {
					t.Errorf("expected config to be created")
					return
				}
				if revisionStatus.Name != "revision-status-1" {
					t.Errorf("expected config to have name 'revision-status-1', got %q", revisionStatus.Name)
				}
				config, hasConfig := createdObjects[1].(*v1.ConfigMap)
				if !hasConfig {
					t.Errorf("expected config to be created")
					return
				}
				if config.Name != "test-config-1" {
					t.Errorf("expected config to have name 'test-config-1', got %q", config.Name)
				}
				secret, hasSecret := createdObjects[2].(*v1.Secret)
				if !hasSecret {
					t.Errorf("expected secret to be created")
					return
				}
				if secret.Name != "test-secret-1" {
					t.Errorf("expected secret to have name 'test-secret-1', got %q", secret.Name)
				}
			},
		},
		{
			testName:        "latest-revision-current",
			targetNamespace: targetNamespace,
			staticPodOperatorClient: v1helpers.NewFakeStaticPodOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
				&operatorv1.OperatorStatus{},
				&operatorv1.StaticPodOperatorSpec{
					OperatorSpec: operatorv1.OperatorSpec{
						ManagementState: operatorv1.Managed,
					},
				},
				&operatorv1.StaticPodOperatorStatus{
					LatestAvailableRevision: 1,
					NodeStatuses: []operatorv1.NodeStatus{
						{
							NodeName:        "test-node-1",
							CurrentRevision: 0,
							TargetRevision:  0,
						},
					},
				},
				nil,
			),
			startingObjects: []runtime.Object{
				&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test-secret", Namespace: targetNamespace}},
				&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test-secret-1", Namespace: targetNamespace}},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: targetNamespace}},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-config-1", Namespace: targetNamespace}},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-1", Namespace: targetNamespace}},
			},
			testConfigs: []RevisionResource{{Name: "test-config"}},
			testSecrets: []RevisionResource{{Name: "test-secret"}},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				createdObjects := filterCreateActions(actions)
				if createdObjectCount := len(createdObjects); createdObjectCount != 0 {
					t.Errorf("expected no objects to be created, got %d", createdObjectCount)
				}
			},
		},
		{
			testName:        "latest-revision-current-optionals-missing",
			targetNamespace: targetNamespace,
			staticPodOperatorClient: v1helpers.NewFakeStaticPodOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
				&operatorv1.OperatorStatus{},
				&operatorv1.StaticPodOperatorSpec{
					OperatorSpec: operatorv1.OperatorSpec{
						ManagementState: operatorv1.Managed,
					},
				},
				&operatorv1.StaticPodOperatorStatus{
					LatestAvailableRevision: 1,
					NodeStatuses: []operatorv1.NodeStatus{
						{
							NodeName:        "test-node-1",
							CurrentRevision: 0,
							TargetRevision:  0,
						},
					},
				},
				nil,
			),
			startingObjects: []runtime.Object{
				&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test-secret", Namespace: targetNamespace}},
				&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test-secret-1", Namespace: targetNamespace}},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: targetNamespace}},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-config-1", Namespace: targetNamespace}},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-1", Namespace: targetNamespace}},
			},
			testConfigs: []RevisionResource{{Name: "test-config"}, {Name: "test-config-opt", Optional: true}},
			testSecrets: []RevisionResource{{Name: "test-secret"}, {Name: "test-secret-opt", Optional: true}},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				createdObjects := filterCreateActions(actions)
				if createdObjectCount := len(createdObjects); createdObjectCount != 0 {
					t.Errorf("expected no objects to be created, got %d", createdObjectCount)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset(tc.startingObjects...)
			eventRecorder := events.NewRecorder(kubeClient.CoreV1().Events("test"), "test-operator", &v1.ObjectReference{})

			c := NewRevisionController(
				tc.targetNamespace,
				tc.testConfigs,
				tc.testSecrets,
				informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace(tc.targetNamespace)),
				tc.staticPodOperatorClient,
				kubeClient.CoreV1(),
				kubeClient.CoreV1(),
				eventRecorder,
			)
			syncErr := c.sync()
			if tc.validateStatus != nil {
				_, status, _, _ := tc.staticPodOperatorClient.GetStaticPodOperatorState()
				tc.validateStatus(t, status)
			}
			if tc.validateActions != nil {
				tc.validateActions(t, kubeClient.Actions())
			}
			if syncErr != nil {
				if !strings.Contains(syncErr.Error(), tc.expectSyncError) {
					t.Errorf("expected %q string in error %q", tc.expectSyncError, syncErr.Error())
				}
				return
			}
			if syncErr == nil && len(tc.expectSyncError) != 0 {
				t.Errorf("expected %v error, got none", tc.expectSyncError)
				return
			}
		})
	}
}
