package monitorapi

import "testing"

func TestGetNodeRoles(t *testing.T) {
	var testCases = []struct {
		event    Interval
		expected string
	}{
		{
			event: Interval{
				Condition: Condition{},
			},
			expected: "",
		},
		{
			event: Interval{
				Condition: Condition{
					Message: Message{Annotations: map[AnnotationKey]string{AnnotationRoles: "master"}},
				},
			},
			expected: "master",
		},
		{
			event: Interval{
				Condition: Condition{
					Message: Message{Annotations: map[AnnotationKey]string{AnnotationRoles: "worker"}},
				},
			},
			expected: "worker",
		},
		{
			event: Interval{
				Condition: Condition{
					Message: Message{Annotations: map[AnnotationKey]string{AnnotationRoles: "master,worker"}},
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
