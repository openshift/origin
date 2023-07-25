package monitorapi

import "testing"

func TestGetNodeRoles(t *testing.T) {
	var testCases = []struct {
		event    EventInterval
		expected string
	}{
		{
			event: EventInterval{
				Condition: Condition{
					Message: "",
				},
			},
			expected: "",
		},
		{
			event: EventInterval{
				Condition: Condition{
					Message: "roles/master",
				},
			},
			expected: "master",
		},
		{
			event: EventInterval{
				Condition: Condition{
					Message: "roles/worker",
				},
			},
			expected: "worker",
		},
		{
			event: EventInterval{
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
