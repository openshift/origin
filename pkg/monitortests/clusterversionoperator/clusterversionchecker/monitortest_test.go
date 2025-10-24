package clusterversionchecker

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func Test_parseClusterOperatorNames(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		reason      string
		message     string
		expected    sets.Set[string]
		expectedErr error
	}{
		{
			name:        "unexpected",
			reason:      "reason",
			message:     "unexpected",
			expectedErr: fmt.Errorf("failed to parse cluster operator names from %q", "changed to Some=Unknown: reason: unexpected"),
		},
		{
			name:        "legit waiting on",
			message:     "Working towards 1.2.3: waiting on co-not-timeout",
			expectedErr: fmt.Errorf("failed to parse cluster operator names from %q", "changed to Some=Unknown: Working towards 1.2.3: waiting on co-not-timeout"),
		},
		{
			name:     "one CO timeout",
			reason:   "SlowClusterOperator",
			message:  "waiting on network over 30 minutes which is longer than expected",
			expected: sets.New[string]("network"),
		},
		{
			name:     "mco timeout",
			reason:   "SlowClusterOperator",
			message:  "waiting on machine-config over 90 minutes which is longer than expected",
			expected: sets.New[string]("machine-config"),
		},
		{
			name:     "mco timeout",
			reason:   "SlowClusterOperator",
			message:  "waiting on machine-config over 90 minutes which is longer than expected",
			expected: sets.New[string]("machine-config"),
		},
		{
			name:     "two COs timeout",
			reason:   "SlowClusterOperator",
			message:  "waiting on console, network over 30 minutes which is longer than expected",
			expected: sets.New[string]("console", "network"),
		},
		{
			name:     "one CO and mco timeout",
			reason:   "SlowClusterOperator",
			message:  "waiting on network over 30 minutes and machine-config over 90 minutes which is longer than expected",
			expected: sets.New[string]("machine-config", "network"),
		},
		{
			name:     "three COs timeout",
			reason:   "SlowClusterOperator",
			message:  "waiting on console, network over 30 minutes and machine-config over 90 minutes which is longer than expected",
			expected: sets.New[string]("machine-config", "console", "network"),
		},
		{
			name:        "unknown operators",
			reason:      "SlowClusterOperator",
			message:     "waiting on unknown, bar, network over 30 minutes and machine-config over 90 minutes which is longer than expected",
			expectedErr: fmt.Errorf(`found unknown operator names "bar, unknown" from "changed to Some=Unknown: SlowClusterOperator: waiting on unknown, bar, network over 30 minutes and machine-config over 90 minutes which is longer than expected"`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msg := monitorapi.GetOperatorConditionHumanMessage(&configv1.ClusterOperatorStatusCondition{
				Type:    "Some",
				Status:  configv1.ConditionUnknown,
				Message: tc.message,
				Reason:  tc.reason,
			}, "changed to ")
			actual, actuallErr := parseClusterOperatorNames(msg)
			if diff := cmp.Diff(tc.expectedErr, actuallErr, cmp.FilterValues(func(x, y interface{}) bool {
				_, ok1 := x.(error)
				_, ok2 := y.(error)
				return ok1 && ok2
			}, cmp.Comparer(func(x, y interface{}) bool {
				xe := x.(error)
				ye := y.(error)
				if xe == nil || ye == nil {
					return xe == nil && ye == nil
				}
				return xe.Error() == ye.Error()
			}))); diff != "" {
				t.Errorf("unexpected error (-want +got):\n%s", diff)
			}

			if actuallErr == nil {
				if diff := cmp.Diff(tc.expected, actual); diff != "" {
					t.Errorf("unexpected result (-want +got):\n%s", diff)
				}
			}
		})
	}
}
