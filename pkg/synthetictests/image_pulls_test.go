package synthetictests

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func Test_testRequiredInstallerResourcesMissing(t *testing.T) {
	tests := []struct {
		name    string
		message string
		kind    string
	}{
		{
			name:    "Test doesn't match but results in passing junit",
			message: "ns/openshift-etcd-operator deployment/etcd-operator - reason/RequiredInstallerYadaMissing secrets: etcd-all-certs-3 (25 times)",
			kind:    "pass",
		},
		{
			name:    "Test failing case",
			message: "ns/openshift-etcd-operator deployment/etcd-operator - reason/RequiredInstallerResourcesMissing secrets: etcd-all-certs-3 (21 times)",
			kind:    "fail",
		},
		{
			name:    "Test flaking case",
			message: "ns/openshift-etcd-operator deployment/etcd-operator - reason/RequiredInstallerResourcesMissing secrets: etcd-all-certs-3 (16 times)",
			kind:    "flake",
		},
		{
			name:    "Test passing case",
			message: "ns/openshift-etcd-operator deployment/etcd-operator - reason/RequiredInstallerResourcesMissing secrets: etcd-all-certs-3 (7 times)",
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
			junit_tests := testRequiredInstallerResourcesMissing(e)
			switch tt.kind {
			case "pass":
				if len(junit_tests) != 1 {
					t.Errorf("This should've been a single passing test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) != 0 {
					t.Errorf("This should've been a pass, but got %s", junit_tests[0].SystemErr)
				}
			case "fail":
				if len(junit_tests) != 1 {
					t.Errorf("This should've been a single failing test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) == 0 {
					t.Error("This should've been a failure but got no output")
				}
			case "flake":
				if len(junit_tests) != 2 {
					t.Errorf("This should've been a two tests as flake, but got %d tests", len(junit_tests))
				}
			default:
				t.Errorf("Unknown test kind")
			}

		})
	}
}
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
					t.Errorf("This should've been a single passing test, but got %d tests", len(junitTests))
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
				t.Errorf("Unknown test kind")
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
					t.Errorf("This should've been a single passing test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) != 0 {
					t.Errorf("This should've been a pass, but got %s", junit_tests[0].SystemErr)
				}
			case "fail":
				if len(junit_tests) != 1 {
					t.Errorf("This should've been a single failing test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) == 0 {
					t.Error("This should've been a failure but got no output")
				}
			case "flake":
				if len(junit_tests) != 2 {
					t.Errorf("This should've been a two tests as flake, but got %d tests", len(junit_tests))
				}
			default:
				t.Errorf("Unknown test kind")
			}

		})
	}
}
