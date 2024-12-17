package installerpod

import (
	"testing"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	corev1 "k8s.io/api/core/v1"
)

func TestHost(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "installer-9-ci-op-54qd4d73-03fd1-cl265-master-0",
			expected: "ci-op-54qd4d73-03fd1-cl265-master-0",
		},
		{
			name:     "installer-9-retry-3-ci-op-54qd4d73-03fd1-cl265-master-0",
			expected: "ci-op-54qd4d73-03fd1-cl265-master-0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			interval := monitorapi.Interval{
				Condition: monitorapi.Condition{
					Locator: monitorapi.NewLocator().KubeEvent(&corev1.Event{
						InvolvedObject: corev1.ObjectReference{
							Kind: "Pod",
							Name: test.name,
						},
					}),
				},
			}

			t.Logf("interval: %+v", interval)

			if want, got := test.expected, host(interval); want != got {
				t.Errorf("expected host name: %q, but got: %q", want, got)
			}
		})
	}
}
