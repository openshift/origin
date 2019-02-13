package v1helpers

import (
	"reflect"
	"strings"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestGetStatusConditionDiff(t *testing.T) {
	tests := []struct {
		name             string
		newConditions    []configv1.ClusterOperatorStatusCondition
		oldConditions    []configv1.ClusterOperatorStatusCondition
		expectedMessages []string
	}{
		{
			name: "new condition",
			newConditions: []configv1.ClusterOperatorStatusCondition{
				{
					Type:    configv1.RetrievedUpdates,
					Status:  configv1.ConditionTrue,
					Message: "test",
				},
			},
			expectedMessages: []string{`RetrievedUpdates set to True ("test")`},
		},
		{
			name: "condition status change",
			newConditions: []configv1.ClusterOperatorStatusCondition{
				{
					Type:    configv1.RetrievedUpdates,
					Status:  configv1.ConditionFalse,
					Message: "test",
				},
			},
			oldConditions: []configv1.ClusterOperatorStatusCondition{
				{
					Type:    configv1.RetrievedUpdates,
					Status:  configv1.ConditionTrue,
					Message: "test",
				},
			},
			expectedMessages: []string{`RetrievedUpdates changed from True to False ("test")`},
		},
		{
			name: "condition message change",
			newConditions: []configv1.ClusterOperatorStatusCondition{
				{
					Type:    configv1.RetrievedUpdates,
					Status:  configv1.ConditionTrue,
					Message: "foo",
				},
			},
			oldConditions: []configv1.ClusterOperatorStatusCondition{
				{
					Type:    configv1.RetrievedUpdates,
					Status:  configv1.ConditionTrue,
					Message: "bar",
				},
			},
			expectedMessages: []string{`RetrievedUpdates message changed from "bar" to "foo"`},
		},
		{
			name: "condition message deleted",
			oldConditions: []configv1.ClusterOperatorStatusCondition{
				{
					Type:    configv1.RetrievedUpdates,
					Status:  configv1.ConditionTrue,
					Message: "test",
				},
			},
			expectedMessages: []string{"RetrievedUpdates was removed"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := GetStatusDiff(configv1.ClusterOperatorStatus{Conditions: test.oldConditions}, configv1.ClusterOperatorStatus{Conditions: test.newConditions})
			if !reflect.DeepEqual(test.expectedMessages, strings.Split(result, ",")) {
				t.Errorf("expected %#v, got %#v", test.expectedMessages, result)
			}
		})
	}
}
