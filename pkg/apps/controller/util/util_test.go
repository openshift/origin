package util

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appstest "github.com/openshift/origin/pkg/apps/apis/apps/internaltest"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestPodName(t *testing.T) {
	deployment := &kapi.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testName",
		},
	}
	expected := "testName-deploy"
	actual := DeployerPodNameForDeployment(deployment.Name)
	if expected != actual {
		t.Errorf("Unexpected pod name for deployment. Expected: %s Got: %s", expected, actual)
	}
}

func TestCanTransitionPhase(t *testing.T) {
	tests := []struct {
		name          string
		current, next appsapi.DeploymentStatus
		expected      bool
	}{
		{
			name:     "New->New",
			current:  appsapi.DeploymentStatusNew,
			next:     appsapi.DeploymentStatusNew,
			expected: false,
		},
		{
			name:     "New->Pending",
			current:  appsapi.DeploymentStatusNew,
			next:     appsapi.DeploymentStatusPending,
			expected: true,
		},
		{
			name:     "New->Running",
			current:  appsapi.DeploymentStatusNew,
			next:     appsapi.DeploymentStatusRunning,
			expected: true,
		},
		{
			name:     "New->Complete",
			current:  appsapi.DeploymentStatusNew,
			next:     appsapi.DeploymentStatusComplete,
			expected: true,
		},
		{
			name:     "New->Failed",
			current:  appsapi.DeploymentStatusNew,
			next:     appsapi.DeploymentStatusFailed,
			expected: true,
		},
		{
			name:     "Pending->New",
			current:  appsapi.DeploymentStatusPending,
			next:     appsapi.DeploymentStatusNew,
			expected: false,
		},
		{
			name:     "Pending->Pending",
			current:  appsapi.DeploymentStatusPending,
			next:     appsapi.DeploymentStatusPending,
			expected: false,
		},
		{
			name:     "Pending->Running",
			current:  appsapi.DeploymentStatusPending,
			next:     appsapi.DeploymentStatusRunning,
			expected: true,
		},
		{
			name:     "Pending->Failed",
			current:  appsapi.DeploymentStatusPending,
			next:     appsapi.DeploymentStatusFailed,
			expected: true,
		},
		{
			name:     "Pending->Complete",
			current:  appsapi.DeploymentStatusPending,
			next:     appsapi.DeploymentStatusComplete,
			expected: true,
		},
		{
			name:     "Running->New",
			current:  appsapi.DeploymentStatusRunning,
			next:     appsapi.DeploymentStatusNew,
			expected: false,
		},
		{
			name:     "Running->Pending",
			current:  appsapi.DeploymentStatusRunning,
			next:     appsapi.DeploymentStatusPending,
			expected: false,
		},
		{
			name:     "Running->Running",
			current:  appsapi.DeploymentStatusRunning,
			next:     appsapi.DeploymentStatusRunning,
			expected: false,
		},
		{
			name:     "Running->Failed",
			current:  appsapi.DeploymentStatusRunning,
			next:     appsapi.DeploymentStatusFailed,
			expected: true,
		},
		{
			name:     "Running->Complete",
			current:  appsapi.DeploymentStatusRunning,
			next:     appsapi.DeploymentStatusComplete,
			expected: true,
		},
		{
			name:     "Complete->New",
			current:  appsapi.DeploymentStatusComplete,
			next:     appsapi.DeploymentStatusNew,
			expected: false,
		},
		{
			name:     "Complete->Pending",
			current:  appsapi.DeploymentStatusComplete,
			next:     appsapi.DeploymentStatusPending,
			expected: false,
		},
		{
			name:     "Complete->Running",
			current:  appsapi.DeploymentStatusComplete,
			next:     appsapi.DeploymentStatusRunning,
			expected: false,
		},
		{
			name:     "Complete->Failed",
			current:  appsapi.DeploymentStatusComplete,
			next:     appsapi.DeploymentStatusFailed,
			expected: false,
		},
		{
			name:     "Complete->Complete",
			current:  appsapi.DeploymentStatusComplete,
			next:     appsapi.DeploymentStatusComplete,
			expected: false,
		},
		{
			name:     "Failed->New",
			current:  appsapi.DeploymentStatusFailed,
			next:     appsapi.DeploymentStatusNew,
			expected: false,
		},
		{
			name:     "Failed->Pending",
			current:  appsapi.DeploymentStatusFailed,
			next:     appsapi.DeploymentStatusPending,
			expected: false,
		},
		{
			name:     "Failed->Running",
			current:  appsapi.DeploymentStatusFailed,
			next:     appsapi.DeploymentStatusRunning,
			expected: false,
		},
		{
			name:     "Failed->Complete",
			current:  appsapi.DeploymentStatusFailed,
			next:     appsapi.DeploymentStatusComplete,
			expected: false,
		},
		{
			name:     "Failed->Failed",
			current:  appsapi.DeploymentStatusFailed,
			next:     appsapi.DeploymentStatusFailed,
			expected: false,
		},
	}

	for _, test := range tests {
		got := CanTransitionPhase(test.current, test.next)
		if got != test.expected {
			t.Errorf("%s: expected %t, got %t", test.name, test.expected, got)
		}
	}
}

var (
	now     = metav1.Now()
	later   = metav1.Time{Time: now.Add(time.Minute)}
	earlier = metav1.Time{Time: now.Add(-time.Minute)}

	condProgressing = func() appsapi.DeploymentCondition {
		return appsapi.DeploymentCondition{
			Type:               appsapi.DeploymentProgressing,
			Status:             kapi.ConditionTrue,
			LastTransitionTime: now,
		}
	}

	condProgressingDifferentTime = func() appsapi.DeploymentCondition {
		return appsapi.DeploymentCondition{
			Type:               appsapi.DeploymentProgressing,
			Status:             kapi.ConditionTrue,
			LastTransitionTime: later,
		}
	}

	condProgressingDifferentReason = func() appsapi.DeploymentCondition {
		return appsapi.DeploymentCondition{
			Type:               appsapi.DeploymentProgressing,
			Status:             kapi.ConditionTrue,
			LastTransitionTime: later,
			Reason:             appsapi.NewReplicationControllerReason,
		}
	}

	condNotProgressing = func() appsapi.DeploymentCondition {
		return appsapi.DeploymentCondition{
			Type:               appsapi.DeploymentProgressing,
			Status:             kapi.ConditionFalse,
			LastUpdateTime:     earlier,
			LastTransitionTime: earlier,
		}
	}

	condAvailable = func() appsapi.DeploymentCondition {
		return appsapi.DeploymentCondition{
			Type:   appsapi.DeploymentAvailable,
			Status: kapi.ConditionTrue,
		}
	}
)

func TestGetCondition(t *testing.T) {
	exampleStatus := func() appsapi.DeploymentConfigStatus {
		return appsapi.DeploymentConfigStatus{
			Conditions: []appsapi.DeploymentCondition{condProgressing(), condAvailable()},
		}
	}

	tests := []struct {
		name string

		status     appsapi.DeploymentConfigStatus
		condType   appsapi.DeploymentConditionType
		condStatus kapi.ConditionStatus

		expected bool
	}{
		{
			name: "condition exists",

			status:   exampleStatus(),
			condType: appsapi.DeploymentAvailable,

			expected: true,
		},
		{
			name: "condition does not exist",

			status:   exampleStatus(),
			condType: appsapi.DeploymentReplicaFailure,

			expected: false,
		},
	}

	for _, test := range tests {
		cond := GetDeploymentCondition(test.status, test.condType)
		exists := cond != nil
		if exists != test.expected {
			t.Errorf("%s: expected condition to exist: %t, got: %t", test.name, test.expected, exists)
		}
	}
}

func TestSetCondition(t *testing.T) {
	tests := []struct {
		name string

		status *appsapi.DeploymentConfigStatus
		cond   appsapi.DeploymentCondition

		expectedStatus *appsapi.DeploymentConfigStatus
	}{
		{
			name: "set for the first time",

			status: &appsapi.DeploymentConfigStatus{},
			cond:   condAvailable(),

			expectedStatus: &appsapi.DeploymentConfigStatus{
				Conditions: []appsapi.DeploymentCondition{
					condAvailable(),
				},
			},
		},
		{
			name: "simple set",

			status: &appsapi.DeploymentConfigStatus{
				Conditions: []appsapi.DeploymentCondition{
					condProgressing(),
				},
			},
			cond: condAvailable(),

			expectedStatus: &appsapi.DeploymentConfigStatus{
				Conditions: []appsapi.DeploymentCondition{
					condProgressing(), condAvailable(),
				},
			},
		},
		{
			name: "replace if status changes",

			status: &appsapi.DeploymentConfigStatus{
				Conditions: []appsapi.DeploymentCondition{
					condNotProgressing(),
				},
			},
			cond: condProgressing(),

			expectedStatus: &appsapi.DeploymentConfigStatus{Conditions: []appsapi.DeploymentCondition{condProgressing()}},
		},
		{
			name: "replace if reason changes",

			status: &appsapi.DeploymentConfigStatus{
				Conditions: []appsapi.DeploymentCondition{
					condProgressing(),
				},
			},
			cond: condProgressingDifferentReason(),

			expectedStatus: &appsapi.DeploymentConfigStatus{
				Conditions: []appsapi.DeploymentCondition{
					{
						Type:   appsapi.DeploymentProgressing,
						Status: kapi.ConditionTrue,
						// Note that LastTransitionTime stays the same.
						LastTransitionTime: now,
						// Only the reason changes.
						Reason: appsapi.NewReplicationControllerReason,
					},
				},
			},
		},
		{
			name: "don't replace if status and reason don't change",

			status: &appsapi.DeploymentConfigStatus{
				Conditions: []appsapi.DeploymentCondition{
					condProgressing(),
				},
			},
			cond: condProgressingDifferentTime(),

			expectedStatus: &appsapi.DeploymentConfigStatus{Conditions: []appsapi.DeploymentCondition{condProgressing()}},
		},
	}

	for _, test := range tests {
		t.Logf("running test %q", test.name)
		SetDeploymentCondition(test.status, test.cond)
		if !reflect.DeepEqual(test.status, test.expectedStatus) {
			t.Errorf("expected status: %v, got: %v", test.expectedStatus, test.status)
		}
	}
}

func TestRemoveCondition(t *testing.T) {
	exampleStatus := func() *appsapi.DeploymentConfigStatus {
		return &appsapi.DeploymentConfigStatus{
			Conditions: []appsapi.DeploymentCondition{condProgressing(), condAvailable()},
		}
	}

	tests := []struct {
		name string

		status   *appsapi.DeploymentConfigStatus
		condType appsapi.DeploymentConditionType

		expectedStatus *appsapi.DeploymentConfigStatus
	}{
		{
			name: "remove from empty status",

			status:   &appsapi.DeploymentConfigStatus{},
			condType: appsapi.DeploymentProgressing,

			expectedStatus: &appsapi.DeploymentConfigStatus{},
		},
		{
			name: "simple remove",

			status:   &appsapi.DeploymentConfigStatus{Conditions: []appsapi.DeploymentCondition{condProgressing()}},
			condType: appsapi.DeploymentProgressing,

			expectedStatus: &appsapi.DeploymentConfigStatus{},
		},
		{
			name: "doesn't remove anything",

			status:   exampleStatus(),
			condType: appsapi.DeploymentReplicaFailure,

			expectedStatus: exampleStatus(),
		},
	}

	for _, test := range tests {
		RemoveDeploymentCondition(test.status, test.condType)
		if !reflect.DeepEqual(test.status, test.expectedStatus) {
			t.Errorf("%s: expected status: %v, got: %v", test.name, test.expectedStatus, test.status)
		}
	}
}

func TestRolloutExceededTimeoutSeconds(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name                   string
		config                 *appsapi.DeploymentConfig
		deploymentCreationTime time.Time
		expectTimeout          bool
	}{
		// Recreate strategy with deployment running for 20s (exceeding 10s timeout)
		{
			name: "recreate timeout",
			config: func(timeoutSeconds int64) *appsapi.DeploymentConfig {
				config := appstest.OkDeploymentConfig(1)
				config.Spec.Strategy.RecreateParams.TimeoutSeconds = &timeoutSeconds
				return config
			}(int64(10)),
			deploymentCreationTime: now.Add(-20 * time.Second),
			expectTimeout:          true,
		},
		// Recreate strategy with no timeout
		{
			name: "recreate no timeout",
			config: func(timeoutSeconds int64) *appsapi.DeploymentConfig {
				config := appstest.OkDeploymentConfig(1)
				config.Spec.Strategy.RecreateParams.TimeoutSeconds = &timeoutSeconds
				return config
			}(int64(0)),
			deploymentCreationTime: now.Add(-700 * time.Second),
			expectTimeout:          false,
		},

		// Rolling strategy with deployment running for 20s (exceeding 10s timeout)
		{
			name: "rolling timeout",
			config: func(timeoutSeconds int64) *appsapi.DeploymentConfig {
				config := appstest.OkDeploymentConfig(1)
				config.Spec.Strategy = appstest.OkRollingStrategy()
				config.Spec.Strategy.RollingParams.TimeoutSeconds = &timeoutSeconds
				return config
			}(int64(10)),
			deploymentCreationTime: now.Add(-20 * time.Second),
			expectTimeout:          true,
		},
		// Rolling strategy with deployment with no timeout specified.
		{
			name: "rolling using default timeout",
			config: func(timeoutSeconds int64) *appsapi.DeploymentConfig {
				config := appstest.OkDeploymentConfig(1)
				config.Spec.Strategy = appstest.OkRollingStrategy()
				config.Spec.Strategy.RollingParams.TimeoutSeconds = nil
				return config
			}(0),
			deploymentCreationTime: now.Add(-20 * time.Second),
			expectTimeout:          false,
		},
		// Recreate strategy with deployment with no timeout specified.
		{
			name: "recreate using default timeout",
			config: func(timeoutSeconds int64) *appsapi.DeploymentConfig {
				config := appstest.OkDeploymentConfig(1)
				config.Spec.Strategy.RecreateParams.TimeoutSeconds = nil
				return config
			}(0),
			deploymentCreationTime: now.Add(-20 * time.Second),
			expectTimeout:          false,
		},
		// Custom strategy with deployment with no timeout specified.
		{
			name: "custom using default timeout",
			config: func(timeoutSeconds int64) *appsapi.DeploymentConfig {
				config := appstest.OkDeploymentConfig(1)
				config.Spec.Strategy = appstest.OkCustomStrategy()
				return config
			}(0),
			deploymentCreationTime: now.Add(-20 * time.Second),
			expectTimeout:          false,
		},
		// Custom strategy use default timeout exceeding it.
		{
			name: "custom using default timeout timing out",
			config: func(timeoutSeconds int64) *appsapi.DeploymentConfig {
				config := appstest.OkDeploymentConfig(1)
				config.Spec.Strategy = appstest.OkCustomStrategy()
				return config
			}(0),
			deploymentCreationTime: now.Add(-700 * time.Second),
			expectTimeout:          true,
		},
	}

	for _, tc := range tests {
		config := tc.config
		deployment, err := MakeDeploymentV1FromInternalConfig(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		deployment.ObjectMeta.CreationTimestamp = metav1.Time{Time: tc.deploymentCreationTime}
		gotTimeout := RolloutExceededTimeoutSeconds(config, deployment)
		if tc.expectTimeout && !gotTimeout {
			t.Errorf("[%s]: expected timeout, but got no timeout", tc.name)
		}
		if !tc.expectTimeout && gotTimeout {
			t.Errorf("[%s]: expected no timeout, but got timeout", tc.name)
		}

	}
}
