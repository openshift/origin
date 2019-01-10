package status

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/openshift/library-go/pkg/operator/events"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/client-go/tools/cache"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	fake "github.com/openshift/client-go/config/clientset/versioned/fake"
)

func TestSync(t *testing.T) {

	testCases := []struct {
		conditions            []operatorv1.OperatorCondition
		expectedFailingStatus configv1.ConditionStatus
		expectedMessages      []string
		expectedReason        string
	}{
		{
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeAFailing", Status: operatorv1.ConditionFalse},
			},
			expectedFailingStatus: configv1.ConditionFalse,
		},
		{
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeAFailing", Status: operatorv1.ConditionTrue},
			},
			expectedFailingStatus: configv1.ConditionTrue,
			expectedReason:        "TypeAFailing",
		},
		{
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeAFailing", Status: operatorv1.ConditionTrue, Message: "a message from type a"},
				{Type: "TypeBFailing", Status: operatorv1.ConditionFalse},
			},
			expectedFailingStatus: configv1.ConditionTrue,
			expectedReason:        "TypeAFailing",
			expectedMessages: []string{
				"TypeAFailing: a message from type a",
			},
		},
		{
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeAFailing", Status: operatorv1.ConditionFalse},
				{Type: "TypeBFailing", Status: operatorv1.ConditionTrue, Message: "a message from type b"},
			},
			expectedFailingStatus: configv1.ConditionTrue,
			expectedReason:        "TypeBFailing",
			expectedMessages: []string{
				"TypeBFailing: a message from type b",
			},
		},
		{
			conditions: []operatorv1.OperatorCondition{
				{Type: "TypeAFailing", Status: operatorv1.ConditionFalse},
				{Type: "TypeBFailing", Status: operatorv1.ConditionTrue, Message: "a message from type b\nanother message from type b"},
				{Type: "TypeCFailing", Status: operatorv1.ConditionFalse, Message: "a message from type c"},
				{Type: "TypeDFailing", Status: operatorv1.ConditionTrue, Message: "a message from type d"},
			},
			expectedFailingStatus: configv1.ConditionTrue,
			expectedReason:        "MultipleConditionsFailing",
			expectedMessages: []string{
				"TypeBFailing: a message from type b",
				"TypeBFailing: another message from type b",
				"TypeDFailing: a message from type d",
			},
		},
	}
	for name, tc := range testCases {
		t.Run(fmt.Sprintf("%05d", name), func(t *testing.T) {
			clusterOperatorClient := fake.NewSimpleClientset(&configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: "OPERATOR_NAME", ResourceVersion: "12"},
			})

			statusClient := &statusClient{
				t: t,
				status: operatorv1.OperatorStatus{
					Conditions: tc.conditions,
				},
			}
			controller := &StatusSyncer{
				clusterOperatorName:    "OPERATOR_NAME",
				clusterOperatorClient:  clusterOperatorClient.ConfigV1(),
				operatorStatusProvider: statusClient,
				eventRecorder:          events.NewInMemoryRecorder("status"),
			}
			if err := controller.sync(); err != nil {
				t.Errorf("unexpected sync error: %v", err)
				return
			}
			result, _ := clusterOperatorClient.ConfigV1().ClusterOperators().Get("OPERATOR_NAME", metav1.GetOptions{})
			expected := &configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: "OPERATOR_NAME", ResourceVersion: "12"},
			}

			if tc.expectedFailingStatus != "" {
				condition := configv1.ClusterOperatorStatusCondition{
					Type:   configv1.OperatorFailing,
					Status: configv1.ConditionStatus(string(tc.expectedFailingStatus)),
				}
				if len(tc.expectedMessages) > 0 {
					condition.Message = strings.Join(tc.expectedMessages, "\n")
				}
				if len(tc.expectedReason) > 0 {
					condition.Reason = tc.expectedReason
				}
				expected.Status.Conditions = append(expected.Status.Conditions, condition)
			}
			for i := range result.Status.Conditions {
				result.Status.Conditions[i].LastTransitionTime = metav1.Time{}
			}

			if !reflect.DeepEqual(expected, result) {
				t.Error(diff.ObjectDiff(expected, result))
			}
		})
	}
}

// OperatorStatusProvider
type statusClient struct {
	t      *testing.T
	status operatorv1.OperatorStatus
}

func (c *statusClient) Informer() cache.SharedIndexInformer {
	c.t.Log("Informer called")
	return nil
}

func (c *statusClient) CurrentStatus() (operatorv1.OperatorStatus, error) {
	return c.status, nil
}
