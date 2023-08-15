package legacynetworkmonitortests

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func Test_testErrorUpdatingEndpointSlices(t *testing.T) {
	tests := []struct {
		name    string
		message string
		kind    string
	}{
		{
			name:    "pass",
			message: "reason/FailedToUpdateEndpointSlices Error updating Endpoint Slices for Service openshift-ovn-kubernetes/ovn-kubernetes-master: node \"ip-10-0-168-211.us-east-2.compute.internal\" not found (2 times)",
			kind:    "pass",
		},
		{
			name:    "flake",
			message: "reason/FailedToUpdateEndpointSlices Error updating Endpoint Slices for Service openshift-ovn-kubernetes/ovn-kubernetes-master: node \"ip-10-0-168-211.us-east-2.compute.internal\" not found (24 times)",
			kind:    "flake",
		},
		{
			name:    "flake",
			message: "reason/FailedToUpdateEndpointSlices Error updating Endpoint Slices for Service openshift-ovn-kubernetes/ovn-kubernetes-master: node \"ip-10-0-168-211.us-east-2.compute.internal\" not found (11 times)",
			kind:    "flake",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Message: tt.message,
						Locator: "ns/openshift-ovn-kubernetes service/ovn-kubernetes-master",
					},
					From: time.Unix(1, 0),
					To:   time.Unix(1, 0),
				},
			}
			junit_tests := testErrorUpdatingEndpointSlices(e)
			switch tt.kind {
			case "pass":
				if len(junit_tests) != 1 {
					t.Errorf("This should've been a single passing Test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) != 0 {
					t.Errorf("This should've been a pass, but got %s", junit_tests[0].SystemErr)
				}
			case "fail":
				if len(junit_tests) != 1 {
					t.Errorf("This should've been a single failing Test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) == 0 {
					t.Error("This should've been a failure but got no output")
				}
			case "flake":
				if len(junit_tests) != 2 {
					t.Errorf("This should've been a two tests as flake, but got %d tests", len(junit_tests))
				}
			default:
				t.Errorf("Unknown Test kind")
			}

		})
	}
}
