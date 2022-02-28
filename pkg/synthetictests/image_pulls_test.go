package synthetictests

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
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
			junit_tests := testBackoffPullingRegistryRedhatImage(e)
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
