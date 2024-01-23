package legacynodemonitortests

import (
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/pathologicaleventlibrary"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_testBackoffPullingRegistryRedhatImage(t *testing.T) {
	tests := []struct {
		name     string
		interval monitorapi.Interval
		numTests int
		kind     string
	}{
		{
			name: "Test flake",
			interval: pathologicaleventlibrary.BuildTestDupeKubeEvent("openshift-e2e-loki",
				"loki-promtail-ww2rx",
				"BackOff",
				"Back-off pulling image \"registry.redhat.io/openshift4/ose-oauth-proxy:latest\"",
				6),
			kind: "flake",
		},
		{
			name: "Test fail",
			interval: pathologicaleventlibrary.BuildTestDupeKubeEvent("openshift-e2e-loki",
				"loki-promtail-ww2rx",
				"BackOff",
				"Back-off pulling image \"registry.redhat.io/openshift4/ose-oauth-proxy:latest\"",
				9),
			kind: "fail",
		},
		{
			name: "Test pass",
			interval: pathologicaleventlibrary.BuildTestDupeKubeEvent("openshift-e2e-loki",
				"loki-promtail-grpkm",
				"BackOff",
				"Back-off pulling image \"registry.redhat.io/openshift4/ose-oauth-proxy:latest\"",
				1),
			kind: "pass",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := monitorapi.Intervals{tt.interval}
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
	namespace := "openshift-etcd-operator"
	samplePod := "etcd-operator-6f9b4d9d4f-4q9q8"

	tests := []struct {
		name     string
		interval monitorapi.Interval
		kind     string
	}{
		{
			name: "Test pass case",
			interval: pathologicaleventlibrary.BuildTestDupeKubeEvent(namespace, samplePod,
				"BackOff",
				"Back-off restarting failed container",
				5),
			kind: "pass",
		},
		{
			name: "Test failure case",
			interval: pathologicaleventlibrary.BuildTestDupeKubeEvent(namespace, samplePod,
				"BackOff",
				"Back-off restarting failed container",
				56),
			kind: "fail",
		},
		{
			name: "Test flake case",
			interval: pathologicaleventlibrary.BuildTestDupeKubeEvent(namespace, samplePod,
				"BackOff",
				"Back-off restarting failed container",
				11),
			kind: "flake",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := monitorapi.Intervals{tt.interval}
			junits := testBackoffStartingFailedContainer(e)

			// Find the junit with the namespace of openshift-etcd-operator int the testname
			var testJunits []*junitapi.JUnitTestCase
			for _, j := range junits {
				if strings.Contains(j.Name, namespace) {
					testJunits = append(testJunits, j)
				}
			}
			if len(testJunits) == 0 {
				t.Errorf("We should have at least one junit test for namespace openshift-etcd-operator")
			}
			switch tt.kind {
			case "pass":
				if len(testJunits) != 1 {
					t.Errorf("This should've been a single passing Test, but got %d tests", len(junits))
				}
				if testJunits[0].FailureOutput != nil && len(testJunits[0].FailureOutput.Output) != 0 {
					t.Errorf("This should've been a pass, but got %s", junits[0].SystemErr)
				}
			case "fail":
				if len(testJunits) != 1 {
					t.Errorf("This should've been a single failing Test, but got %d tests", len(junits))
				}
				if testJunits[0].FailureOutput != nil && len(testJunits[0].FailureOutput.Output) == 0 {
					t.Error("This should've been a failure but got no output")
				}
			case "flake":
				if len(testJunits) != 2 {
					t.Errorf("This should've been a two tests as flake, but got %d tests", len(junits))
				}
			default:
				t.Errorf("Unknown Test kind")
			}

		})
	}
}

func Test_testFailedScheduling(t *testing.T) {
	tests := []struct {
		name     string
		interval monitorapi.Interval
		kind     string
	}{
		{
			name: "event count under threshold should pass",
			interval: pathologicaleventlibrary.BuildTestDupeKubeEvent("", "",
				"FailedScheduling",
				"0/6 nodes are available: 3 node(s) didn't match Pod's node affinity/selector, 3 node(s) didn't match pod anti-affinity rules. preemption: 0/6 nodes are available: 3 Preemption is not helpful for scheduling, 3 node(s) didn't match pod anti-affinity rules..",
				2),
			kind: "pass",
		},
		{
			name: "event count over threshold should flake for aws",
			interval: pathologicaleventlibrary.BuildTestDupeKubeEvent("", "",
				"FailedScheduling",
				"0/6 nodes are available: 3 node(s) didn't match Pod's node affinity/selector, 3 node(s) didn't match pod anti-affinity rules. preemption: 0/6 nodes are available: 3 Preemption is not helpful for scheduling, 3 node(s) didn't match pod anti-affinity rules..",
				24),
			kind: "flake",
		},
		{
			name: "event count over threshold should flake for gcp",
			interval: pathologicaleventlibrary.BuildTestDupeKubeEvent("", "",
				"FailedScheduling",
				"0/6 nodes are available: 3 node(s) didn't match Pod's node affinity/selector, 3 node(s) didn't match pod anti-affinity rules. preemption: 0/6 nodes are available: 3 Preemption is not helpful for scheduling, 3 node(s) didn't match pod anti-affinity rules..",
				21),
			kind: "flake",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := monitorapi.Intervals{tt.interval}
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
