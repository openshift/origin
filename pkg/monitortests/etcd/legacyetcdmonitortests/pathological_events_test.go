package legacyetcdmonitortests

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func Test_testOperatorStatusChanged(t *testing.T) {
	tests := []struct {
		name    string
		message string
		kind    string
	}{
		{
			name:    "event count under threshold should pass",
			message: "reason/OperatorStatusChanged Status for clusteroperator/etcd changed: Degraded message changed from \"NodeControllerDegraded: All master nodes are ready/EtcdMembersDegraded: 2 of 3 members are available, ip-10-0-217-93.us-west-1.compute.internal is unhealthy\" to \"NodeControllerDegraded: All master nodes are ready/EtcdMembersDegraded: No unhealthy members found\" (2 times)",
			kind:    "pass",
		},
		{
			name:    "event count over threshold should flake",
			message: "reason/OperatorStatusChanged Status for clusteroperator/etcd changed: Degraded message changed from \"NodeControllerDegraded: All master nodes are ready/EtcdMembersDegraded: 2 of 3 members are available, ip-10-0-217-93.us-west-1.compute.internal is unhealthy\" to \"NodeControllerDegraded: All master nodes are ready/EtcdMembersDegraded: No unhealthy members found\" (24 times)",
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
			junitTests := testOperatorStatusChanged(e)
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
