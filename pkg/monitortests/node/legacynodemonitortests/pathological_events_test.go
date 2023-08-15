package legacynodemonitortests

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_testBackoffPullingRegistryRedhatImage(t *testing.T) {
	tests := []struct {
		name      string
		message   string
		num_tests int
		kind      string
	}{
		{
			name:    "Test flake",
			message: `ns/openshift-e2e-loki pod/loki-promtail-ww2rx node/ip-10-0-157-209.us-east-2.compute.internal reason/BackOff Back-off pulling image "registry.redhat.io/openshift4/ose-oauth-proxy:latest" (6 times)`,
			kind:    "flake",
		},
		{
			name:    "Test fail",
			message: `ns/openshift-e2e-loki pod/loki-promtail-ww2rx node/ip-10-0-157-209.us-east-2.compute.internal reason/BackOff Back-off pulling image "registry.redhat.io/openshift4/ose-oauth-proxy:latest" (9 times)`,
			kind:    "fail",
		},
		{
			name:    "Test pass",
			message: `ns/openshift-e2e-loki pod/loki-promtail-qrpkm node/ip-10-0-240-197.us-east-2.compute.internal reason/BackOff Back-off pulling image "registry.not-redhat.io/openshift4/ose-oauth-proxy:latest" (1 times)`,
			kind:    "pass",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Message: tt.message,
					},
					From: time.Unix(1, 0),
					To:   time.Unix(1, 0),
				},
			}
			junitTests := testBackoffPullingRegistryRedhatImage(e)
			switch tt.kind {
			case "pass":
				if len(junitTests) != 1 {
					t.Errorf("This should've been a single passing Test, but got %d tests", len(junitTests))
				}
				if len(junitTests[0].SystemOut) != 0 {
					t.Errorf("This should've been a pass, but got %s", junitTests[0].SystemErr)
				}
			case "fail":
				// At this time, we always want this case to be a flake; so, this will always flake
				// since failureThreshold is maxInt.
				if len(junitTests) != 2 {
					t.Errorf("This should've been a two tests as flake, but got %d tests", len(junitTests))
				}
			case "flake":
				if len(junitTests) != 2 {
					t.Errorf("This should've been a two tests as flake, but got %d tests", len(junitTests))
				}
			default:
				t.Errorf("Unknown Test kind")
			}
		})
	}
}

func Test_testBackoffStartingFailedContainer(t *testing.T) {
	tests := []struct {
		name    string
		message string
		kind    string
	}{
		{
			name:    "Test pass case",
			message: "reason/BackOff Back-off restarting failed container (5 times)",
			kind:    "pass",
		},
		{
			name:    "Test failure case",
			message: "reason/BackOff Back-off restarting failed container (56 times)",
			kind:    "fail",
		},
		{
			name:    "Test flake case",
			message: "reason/BackOff Back-off restarting failed container (11 times)",
			kind:    "flake",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Message: tt.message,
					},
					From: time.Unix(1, 0),
					To:   time.Unix(1, 0),
				},
			}
			junit_tests := testBackoffStartingFailedContainer(e)
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

func Test_testErrorReconcilingNode(t *testing.T) {
	tests := []struct {
		name    string
		message string
		kind    string
	}{
		{
			name:    "event count under threshold should pass",
			message: "reason/ErrorReconcilingNode roles/worker [k8s.ovn.org/node-chassis-id annotation not found for node ip-10-0-194-138.us-west-1.compute.internal, macAddress annotation not found for node \"ip-10-0-194-138.us-west-1.compute.internal\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ip-10-0-194-138.us-west-1.compute.internal\"] (2 times)",
			kind:    "pass",
		},
		{
			name:    "event count over threshold should flake for vsphere",
			message: "reason/ErrorReconcilingNode roles/worker [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-6bwnrmql-92dbc-5q7zh-worker-0-vlbtl, macAddress annotation not found for node \"ci-op-6bwnrmql-92dbc-5q7zh-worker-0-vlbtl\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-6bwnrmql-92dbc-5q7zh-worker-0-vlbtl\"] (24 times)",
			kind:    "flake",
		},
		{
			name:    "event count over threshold should flake for aws",
			message: "reason/ErrorReconcilingNode roles/worker [k8s.ovn.org/node-chassis-id annotation not found for node ip-10-0-136-47.us-east-2.compute.internal, macAddress annotation not found for node \"ip-10-0-136-47.us-east-2.compute.internal\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ip-10-0-136-47.us-east-2.compute.internal\"] (21 times)",
			kind:    "flake",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Message: tt.message,
					},
					From: time.Unix(1, 0),
					To:   time.Unix(1, 0),
				},
			}
			junitTests := testErrorReconcilingNode(e)
			switch tt.kind {
			case "pass":
				assert.Equal(t, 1, len(junitTests), "This should've been a single passing Test")
				assert.Nil(t, junitTests[0].FailureOutput, "This should've been a pass")
			case "fail":
				require.Equal(t, 1, len(junitTests), "This should've been a single failing Test")
				require.NotEqual(t, 0, len(junitTests[0].SystemOut), "This should've been a failure")
			case "flake":
				assert.Equal(t, 2, len(junitTests), "This should've been two tests as flake")
			default:
				require.Fail(t, "Unknown Test kind")
			}
		})
	}
}

func Test_testFailedScheduling(t *testing.T) {
	tests := []struct {
		name    string
		message string
		kind    string
	}{
		{
			name:    "event count under threshold should pass",
			message: "reason/FailedScheduling 0/6 nodes are available: 3 node(s) didn't match Pod's node affinity/selector, 3 node(s) didn't match pod anti-affinity rules. preemption: 0/6 nodes are available: 3 Preemption is not helpful for scheduling, 3 node(s) didn't match pod anti-affinity rules.. (2 times)",
			kind:    "pass",
		},
		{
			name:    "event count over threshold should flake for aws",
			message: "reason/FailedScheduling 0/6 nodes are available: 3 node(s) didn't match Pod's node affinity/selector, 3 node(s) didn't match pod anti-affinity rules. preemption: 0/6 nodes are available: 3 Preemption is not helpful for scheduling, 3 node(s) didn't match pod anti-affinity rules.. (24 times)",
			kind:    "flake",
		},
		{
			name:    "event count over threshold should flake for gcp",
			message: "reason/FailedScheduling 0/6 nodes are available: 1 node(s) were unschedulable, 2 node(s) didn't match pod anti-affinity rules, 3 node(s) didn't match Pod's node affinity/selector. preemption: 0/6 nodes are available: 2 node(s) didn't match pod anti-affinity rules, 4 Preemption is not helpful for scheduling.. (21 times)",
			kind:    "flake",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Message: tt.message,
					},
					From: time.Unix(1, 0),
					To:   time.Unix(1, 0),
				},
			}
			junitTests := testFailedScheduling(e)
			switch tt.kind {
			case "pass":
				assert.Equal(t, 1, len(junitTests), "This should've been a single passing Test")
				assert.Nil(t, junitTests[0].FailureOutput, "This should've been a pass")
			case "fail":
				require.Equal(t, 1, len(junitTests), "This should've been a single failing Test")
				require.NotEqual(t, 0, len(junitTests[0].SystemOut), "This should've been a failure")
			case "flake":
				assert.Equal(t, 2, len(junitTests), "This should've been two tests as flake")
			default:
				require.Fail(t, "Unknown Test kind")
			}
		})
	}
}
