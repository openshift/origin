package backingresource

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
		createdObjects = append(createdObjects, createAction.GetObject())
	}
	return createdObjects
}

type prependReactorSpec struct {
	verb, resource string
	reaction       clienttesting.ReactionFunc
}

func TestBackingResourceController(t *testing.T) {
	tests := []struct {
		targetNamespace string
		prependReactors []prependReactorSpec
		startingObjects []runtime.Object
		operatorClient  v1helpers.OperatorClient
		validateActions func(t *testing.T, actions []clienttesting.Action)
		validateStatus  func(t *testing.T, status *operatorv1.OperatorStatus)
		expectSyncError string
	}{
		{
			targetNamespace: "successful-create",
			operatorClient: v1helpers.NewFakeOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
				&operatorv1.OperatorStatus{},
				nil,
			),
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				createdObjects := filterCreateActions(actions)
				if createdObjectCount := len(createdObjects); createdObjectCount != 2 {
					t.Errorf("expected 2 objects to be created, got %d", createdObjectCount)
					return
				}
				sa, hasServiceAccount := createdObjects[0].(*v1.ServiceAccount)
				if !hasServiceAccount {
					t.Errorf("expected service account to be created first, got %+v", createdObjects[0])
					return
				}
				if sa.Namespace != "successful-create" {
					t.Errorf("expected that service account to have 'tc-successful-create' namespace, got %q", sa.Namespace)
					return
				}
				if sa.Name != "installer-sa" {
					t.Errorf("expected service account to have name 'installer-sa', got %q", sa.Name)
				}

				crb, hasClusterRoleBinding := createdObjects[1].(*rbacv1.ClusterRoleBinding)
				if !hasClusterRoleBinding {
					t.Errorf("expected cluster role binding as second object, got %+v", createdObjects[1])
				}
				if rbNamespace := crb.Subjects[0].Namespace; rbNamespace != "successful-create" {
					t.Errorf("expected that cluster role binding first subject to have 'tc-successful-create' namespace, got %q", rbNamespace)
					return
				}
				if crb.Name != "system:openshift:operator:successful-create-installer" {
					t.Errorf("expected that cluster role binding name is 'system:openshift:operator:tc-successful-create-installer', got %q", crb.Name)
				}
			},
		},
		{
			targetNamespace: "operator-unmanaged",
			operatorClient: v1helpers.NewFakeOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Unmanaged,
				},
				&operatorv1.OperatorStatus{},
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
			targetNamespace: "service-account-exists",
			startingObjects: []runtime.Object{
				&v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "installer-sa", Namespace: "service-account-exists"}},
			},
			operatorClient: v1helpers.NewFakeOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
				&operatorv1.OperatorStatus{},
				nil,
			),
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				createdObjects := filterCreateActions(actions)
				if createdObjectCount := len(createdObjects); createdObjectCount != 1 {
					t.Errorf("expected only one object to be created, got %d", createdObjectCount)
				}
				crb, hasClusterRoleBinding := createdObjects[0].(*rbacv1.ClusterRoleBinding)
				if !hasClusterRoleBinding {
					t.Errorf("expected cluster role binding as second object, got %+v", createdObjects[0])
				}
				if rbNamespace := crb.Subjects[0].Namespace; rbNamespace != "service-account-exists" {
					t.Errorf("expected that cluster role binding first subject to have 'tc-successful-create' namespace, got %q", rbNamespace)
					return
				}
				if crb.Name != "system:openshift:operator:service-account-exists-installer" {
					t.Errorf("expected that cluster role binding name is 'system:openshift:operator:tc-successful-create-installer', got %q", crb.Name)
				}
			},
		},
		{
			targetNamespace: "resource-apply-failed",
			prependReactors: []prependReactorSpec{
				{
					verb:     "*",
					resource: "serviceaccounts",
					reaction: func(clienttesting.Action) (bool, runtime.Object, error) {
						return true, nil, fmt.Errorf("test error")
					},
				},
			},
			operatorClient: v1helpers.NewFakeOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
				&operatorv1.OperatorStatus{},
				nil,
			),
			expectSyncError: `test error`,
			validateStatus: func(t *testing.T, status *operatorv1.OperatorStatus) {
				if status.Conditions[0].Type != operatorStatusBackingResourceControllerFailing {
					t.Errorf("expected status condition to be failing, got %v", status.Conditions[0].Type)
				}
				if status.Conditions[0].Reason != "Error" {
					t.Errorf("expected status condition reason to be 'Error', got %v", status.Conditions[0].Reason)
				}
				if !strings.Contains(status.Conditions[0].Message, "test error") {
					t.Errorf("expected status condition message to contain 'test error', got: %s", status.Conditions[0].Message)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.targetNamespace, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset(tc.startingObjects...)
			for _, r := range tc.prependReactors {
				kubeClient.PrependReactor(r.verb, r.resource, r.reaction)
			}
			eventRecorder := events.NewInMemoryRecorder("")
			c := NewBackingResourceController(
				tc.targetNamespace,
				tc.operatorClient,
				informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace(tc.targetNamespace)),
				kubeClient,
				eventRecorder,
			)
			syncErr := c.sync()
			if tc.validateStatus != nil {
				_, status, _, _ := tc.operatorClient.GetOperatorState()
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
			tc.validateActions(t, kubeClient.Actions())
		})
	}
}
