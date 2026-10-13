package legacynetworkmonitortests

import (
	"testing"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func Test_testErrorUpdatingEndpointSlices(t *testing.T) {
	tests := []struct {
		name     string
		interval monitorapi.Interval
		kind     string
	}{
		{
			name: "pass",
			interval: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Locator: monitorapi.Locator{
						Keys: map[monitorapi.LocatorKey]string{
							monitorapi.LocatorNamespaceKey: "openshift-ovn-kubernetes",
						},
					},
					Message: monitorapi.Message{
						Reason:       monitorapi.IntervalReason("FailedToUpdateEndpointSlices"),
						HumanMessage: "Error updating Endpoint Slices for Service openshift-ovn-kubernetes/ovn-kubernetes-master: node \"ip-10-0-168-211.us-east-2.compute.internal\" not found",
						Annotations: map[monitorapi.AnnotationKey]string{
							monitorapi.AnnotationCount: "2",
						},
					},
				},
			},
			kind: "pass",
		},
		{
			name: "flake over 20",
			interval: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Locator: monitorapi.Locator{
						Keys: map[monitorapi.LocatorKey]string{
							monitorapi.LocatorNamespaceKey: "openshift-ovn-kubernetes",
						},
					},
					Message: monitorapi.Message{
						Reason:       monitorapi.IntervalReason("FailedToUpdateEndpointSlices"),
						HumanMessage: "Error updating Endpoint Slices for Service openshift-ovn-kubernetes/ovn-kubernetes-master: node \"ip-10-0-168-211.us-east-2.compute.internal\" not found",
						Annotations: map[monitorapi.AnnotationKey]string{
							monitorapi.AnnotationCount: "24",
						},
					},
				},
			},
			kind: "flake",
		},
		{
			name: "flake over 10",
			interval: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Locator: monitorapi.Locator{
						Keys: map[monitorapi.LocatorKey]string{
							monitorapi.LocatorNamespaceKey: "openshift-ovn-kubernetes",
						},
					},
					Message: monitorapi.Message{
						Reason:       monitorapi.IntervalReason("FailedToUpdateEndpointSlices"),
						HumanMessage: "Error updating Endpoint Slices for Service openshift-ovn-kubernetes/ovn-kubernetes-master: node \"ip-10-0-168-211.us-east-2.compute.internal\" not found",
						Annotations: map[monitorapi.AnnotationKey]string{
							monitorapi.AnnotationCount: "11",
						},
					},
				},
			},
			kind: "flake",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := monitorapi.Intervals{tt.interval}
			junits := testErrorUpdatingEndpointSlices(e)
			switch tt.kind {
			case "pass":
				if len(junits) != 1 {
					t.Errorf("This should've been a single passing Test, but got %d tests", len(junits))
				}
				if len(junits[0].SystemOut) != 0 {
					t.Errorf("This should've been a pass, but got %s", junits[0].SystemErr)
				}
			case "fail":
				if len(junits) != 1 {
					t.Errorf("This should've been a single failing Test, but got %d tests", len(junits))
				}
				if len(junits[0].SystemOut) == 0 {
					t.Error("This should've been a failure but got no output")
				}
			case "flake":
				if len(junits) != 2 {
					t.Errorf("This should've been a two tests as flake, but got %d tests", len(junits))
				}
			default:
				t.Errorf("Unknown Test kind")
			}

		})
	}
}
