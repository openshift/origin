package management

import (
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/library-go/pkg/operator/events"
)

func TestOperatorManagementStateController(t *testing.T) {
	testCases := []struct {
		name              string
		initialConditions []operatorv1.OperatorCondition
		managementState   string
		allowUnmanaged    func() bool
		allowRemove       func() bool

		expectedFailingStatus bool
		expectedMessage       string
	}{
		{
			name:            "operator in managed state with no restrictions",
			managementState: string(operatorv1.Managed),
			allowRemove:     func() bool { return true },
			allowUnmanaged:  func() bool { return true },
		},
		{
			name:            "operator in unmanaged state with no restrictions",
			managementState: string(operatorv1.Unmanaged),
			allowRemove:     func() bool { return true },
			allowUnmanaged:  func() bool { return true },
		},
		{
			name:                  "operator in unknown state with no restrictions",
			managementState:       string("UnknownState"),
			expectedFailingStatus: true,
			expectedMessage:       `Unsupported management state "UnknownState" for OPERATOR_NAME operator`,
			allowRemove:           func() bool { return true },
			allowUnmanaged:        func() bool { return true },
		},
		{
			name:                  "operator in unmanaged state with unmanaged not allowed",
			managementState:       string(operatorv1.Unmanaged),
			expectedFailingStatus: true,
			expectedMessage:       `Unmanaged is not supported for OPERATOR_NAME operator`,
			allowRemove:           func() bool { return true },
			allowUnmanaged:        func() bool { return false },
		},
		{
			name:                  "operator in removed state with removed  not allowed",
			managementState:       string(operatorv1.Removed),
			expectedFailingStatus: true,
			expectedMessage:       `Removed is not supported for OPERATOR_NAME operator`,
			allowRemove:           func() bool { return false },
			allowUnmanaged:        func() bool { return false },
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			getAllowedOperatorRemovedState = tc.allowRemove
			getAllowedOperatorUnmanaged = tc.allowUnmanaged

			statusClient := &statusClient{
				t: t,
				spec: operatorv1.OperatorSpec{
					ManagementState: operatorv1.ManagementState(tc.managementState),
				},
				status: operatorv1.OperatorStatus{
					Conditions: tc.initialConditions,
				},
			}
			controller := &ManagementStateController{
				operatorName:   "OPERATOR_NAME",
				operatorClient: statusClient,
				eventRecorder:  events.NewInMemoryRecorder("status"),
			}
			if err := controller.sync(); err != nil {
				t.Errorf("unexpected sync error: %v", err)
				return
			}

			_, result, _, _ := statusClient.GetOperatorState()

			if tc.expectedFailingStatus && result.Conditions[0].Type == "ManagementStateFailing" && result.Conditions[0].Status == operatorv1.ConditionFalse {
				t.Errorf("expected failing conditions")
				return
			}

			if !tc.expectedFailingStatus && result.Conditions[0].Type == "ManagementStateFailing" && result.Conditions[0].Status != operatorv1.ConditionFalse {
				t.Errorf("unexpected failing conditions: %#v", result.Conditions)
				return
			}

			if tc.expectedFailingStatus {
				if result.Conditions[0].Message != tc.expectedMessage {
					t.Errorf("expected message %q, got %q", result.Conditions[0].Message, tc.expectedMessage)
				}
			}
		})
	}
}

// OperatorStatusProvider
type statusClient struct {
	t      *testing.T
	spec   operatorv1.OperatorSpec
	status operatorv1.OperatorStatus
}

func (c *statusClient) Informer() cache.SharedIndexInformer {
	c.t.Log("Informer called")
	return nil
}

func (c *statusClient) GetOperatorState() (*operatorv1.OperatorSpec, *operatorv1.OperatorStatus, string, error) {
	return &c.spec, &c.status, "", nil
}

func (c *statusClient) UpdateOperatorSpec(string, *operatorv1.OperatorSpec) (spec *operatorv1.OperatorSpec, resourceVersion string, err error) {
	panic("missing")
}

func (c *statusClient) UpdateOperatorStatus(version string, s *operatorv1.OperatorStatus) (status *operatorv1.OperatorStatus, err error) {
	c.status = *s
	return &c.status, nil
}
