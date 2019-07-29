package genericoperatorclient

import (
	"reflect"
	"testing"

	"k8s.io/utils/diff"

	operatorv1 "github.com/openshift/api/operator/v1"
)

func TestSetOperatorSpecFromUnstructured(t *testing.T) {
	tests := []struct {
		name string

		in       map[string]interface{}
		spec     *operatorv1.OperatorSpec
		expected map[string]interface{}
	}{
		{
			name: "keep-unknown",
			in: map[string]interface{}{
				"spec": map[string]interface{}{
					"non-standard-field": "value",
				},
			},
			spec: &operatorv1.OperatorSpec{
				LogLevel: operatorv1.Trace,
			},
			expected: map[string]interface{}{
				"spec": map[string]interface{}{
					"non-standard-field": "value",
					"logLevel":           "Trace",
					"managementState":    "",
					"operatorLogLevel":   "",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := setOperatorSpecFromUnstructured(test.in, test.spec)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(test.in, test.expected) {
				t.Errorf(diff.ObjectDiff(test.in, test.expected))
			}
		})
	}
}

func TestSetOperatorStatusFromUnstructured(t *testing.T) {
	tests := []struct {
		name string

		in       map[string]interface{}
		status   *operatorv1.OperatorStatus
		expected map[string]interface{}
	}{
		{
			name: "keep-unknown",
			in: map[string]interface{}{
				"status": map[string]interface{}{
					"non-standard-field": "value",
				},
			},
			status: &operatorv1.OperatorStatus{
				Conditions: []operatorv1.OperatorCondition{
					{
						Type: "Degraded",
					},
				},
			},
			expected: map[string]interface{}{
				"status": map[string]interface{}{
					"non-standard-field": "value",
					"conditions": []interface{}{
						map[string]interface{}{
							"lastTransitionTime": nil,
							"status":             "",
							"type":               "Degraded",
						},
					},
					"readyReplicas": int64(0),
				},
			},
		},
		{
			name: "replace-condition",
			in: map[string]interface{}{
				"status": map[string]interface{}{
					"non-standard-field": "value",
					"conditions": []interface{}{
						map[string]interface{}{
							"lastTransitionTime": nil,
							"status":             "",
							"type":               "overwriteme",
						},
					},
				},
			},
			status: &operatorv1.OperatorStatus{
				Conditions: []operatorv1.OperatorCondition{
					{
						Type: "Degraded",
					},
				},
			},
			expected: map[string]interface{}{
				"status": map[string]interface{}{
					"non-standard-field": "value",
					"conditions": []interface{}{
						map[string]interface{}{
							"lastTransitionTime": nil,
							"status":             "",
							"type":               "Degraded",
						},
					},
					"readyReplicas": int64(0),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := setOperatorStatusFromUnstructured(test.in, test.status)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(test.in, test.expected) {
				t.Errorf(diff.ObjectGoPrintDiff(test.in, test.expected))
			}
		})
	}
}
