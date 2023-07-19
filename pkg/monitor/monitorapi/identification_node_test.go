package monitorapi

import "testing"

func TestGetNodeRoles(t *testing.T) {
	var testCases = []struct {
		event    Interval
		expected string
	}{
		{
			event: Interval{
				Condition: Condition{
					Message: "",
				},
			},
			expected: "",
		},
		{
			event: Interval{
				Condition: Condition{
					Message: "roles/master",
				},
			},
			expected: "master",
		},
		{
			event: Interval{
				Condition: Condition{
					Message: "roles/worker",
				},
			},
			expected: "worker",
		},
		{
			event: Interval{
				Condition: Condition{
					Message: "roles/master,worker",
				},
			},
			expected: "master,worker",
		},
	}

	for i, tc := range testCases {
		if actual := GetNodeRoles(tc.event); tc.expected != actual {
			t.Errorf("mismatch node roles. test case:#%d expected: %s, actual: %s", i, tc.expected, actual)
		}
	}
}
