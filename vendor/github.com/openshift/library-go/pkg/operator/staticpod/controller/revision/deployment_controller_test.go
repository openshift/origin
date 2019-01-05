package revision

import (
	"strings"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/common"
)

func filterCreateActions(actions []clienttesting.Action) []runtime.Object {
	var createdObjects []runtime.Object
	for _, a := range actions {
		createAction, isCreate := a.(clienttesting.CreateAction)
		if !isCreate {
			continue
		}
		createdObjects = append(createdObjects, createAction.GetObject())
	}
	return createdObjects
}

func TestRevisionController(t *testing.T) {
	tests := []struct {
		targetNamespace         string
		testSecrets             []string
		testConfigs             []string
		startingObjects         []runtime.Object
		staticPodOperatorClient common.OperatorClient
		validateActions         func(t *testing.T, actions []clienttesting.Action)
		validateStatus          func(t *testing.T, status *operatorv1.StaticPodOperatorStatus)
		expectSyncError         string
	}{
		{
			targetNamespace: "operator-unmanaged",
			staticPodOperatorClient: common.NewFakeStaticPodOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Unmanaged,
				},
				&operatorv1.OperatorStatus{},
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
			targetNamespace: "missing-source-resources",
			staticPodOperatorClient: common.NewFakeStaticPodOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
				&operatorv1.OperatorStatus{},
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
			testConfigs:     []string{"test-config"},
			testSecrets:     []string{"test-secret"},
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
			targetNamespace: "copy-resources",
			staticPodOperatorClient: common.NewFakeStaticPodOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
				&operatorv1.OperatorStatus{},
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
				&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test-secret", Namespace: "copy-resources"}},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: "copy-resources"}},
			},
			testConfigs: []string{"test-config"},
			testSecrets: []string{"test-secret"},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				createdObjects := filterCreateActions(actions)
				if createdObjectCount := len(createdObjects); createdObjectCount != 2 {
					t.Errorf("expected 2 objects to be created, got %d", createdObjectCount)
					return
				}
				config, hasConfig := createdObjects[0].(*v1.ConfigMap)
				if !hasConfig {
					t.Errorf("expected config to be created")
					return
				}
				if config.Name != "test-config-1" {
					t.Errorf("expected config to have name 'test-config-1', got %q", config.Name)
				}
				secret, hasSecret := createdObjects[1].(*v1.Secret)
				if !hasSecret {
					t.Errorf("expected secret to be created")
					return
				}
				if secret.Name != "test-secret-1" {
					t.Errorf("expected secret to have name 'test-secret-1', got %q", secret.Name)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.targetNamespace, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset(tc.startingObjects...)
			eventRecorder := events.NewRecorder(kubeClient.CoreV1().Events("test"), "test-operator", &v1.ObjectReference{})

			c := NewRevisionController(
				tc.targetNamespace,
				tc.testConfigs,
				tc.testSecrets,
				informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace(tc.targetNamespace)),
				tc.staticPodOperatorClient,
				kubeClient,
				eventRecorder,
			)
			syncErr := c.sync()
			if tc.validateStatus != nil {
				_, status, _, _ := tc.staticPodOperatorClient.Get()
				tc.validateStatus(t, status)
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
