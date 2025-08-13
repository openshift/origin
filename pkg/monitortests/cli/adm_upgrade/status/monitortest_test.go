package admupgradestatus

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

var exampleOutput = `Unable to fetch alerts, ignoring alerts in 'Update Health':  failed to get alerts from Thanos: no token is currently in use for this session
= Control Plane =
Assessment:      Progressing
Target Version:  4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest (from 4.20.0-0.ci-2025-08-13-114210-test-ci-op-njttt0ww-initial)
Updating:        kube-apiserver
Completion:      6% (2 operators updated, 1 updating, 31 waiting)
Duration:        8m57s (Est. Time Remaining: 1h9m)
Operator Health: 34 Healthy

Updating Cluster Operators
NAME             SINCE   REASON          MESSAGE
kube-apiserver   7m27s   NodeInstaller   NodeInstallerProgressing: 1 node is at revision 7; 2 nodes are at revision 8

Control Plane Nodes
NAME                                        ASSESSMENT   PHASE     VERSION                                                     EST   MESSAGE
ip-10-0-111-19.us-west-1.compute.internal   Outdated     Pending   4.20.0-0.ci-2025-08-13-114210-test-ci-op-njttt0ww-initial   ?
ip-10-0-53-218.us-west-1.compute.internal   Outdated     Pending   4.20.0-0.ci-2025-08-13-114210-test-ci-op-njttt0ww-initial   ?
ip-10-0-99-189.us-west-1.compute.internal   Outdated     Pending   4.20.0-0.ci-2025-08-13-114210-test-ci-op-njttt0ww-initial   ?

= Worker Upgrade =

WORKER POOL   ASSESSMENT   COMPLETION   STATUS
worker        Pending      0% (0/3)     3 Available, 0 Progressing, 0 Draining

Worker Pool Nodes: worker
NAME                                         ASSESSMENT   PHASE     VERSION                                                     EST   MESSAGE
ip-10-0-0-72.us-west-1.compute.internal      Outdated     Pending   4.20.0-0.ci-2025-08-13-114210-test-ci-op-njttt0ww-initial   ?
ip-10-0-100-255.us-west-1.compute.internal   Outdated     Pending   4.20.0-0.ci-2025-08-13-114210-test-ci-op-njttt0ww-initial   ?
ip-10-0-106-212.us-west-1.compute.internal   Outdated     Pending   4.20.0-0.ci-2025-08-13-114210-test-ci-op-njttt0ww-initial   ?

= Update Health =
SINCE   LEVEL   IMPACT   MESSAGE
8m57s   Info    None     Update is proceeding well`

var badOutput = `= Control Plane =
Assessment:      Progressing
Target Version:  4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest (from 4.20.0-0.ci-2025-08-13-114210-test-ci-op-njttt0ww-initial)
Completion:      6% (2 operators updated, 1 updating, 31 waiting)
Duration:        8m57s (Est. Time Remaining: 1h9m)
Operator Health: 34 Healthy

Control Plane Nodes
NAME                                        ASSESSMENT   PHASE     VERSION                                                     EST   MESSAGE
ip-10-0-111-19.us-west-1.compute.internal   Outdated     Pending   4.20.0-0.ci-2025-08-13-114210-test-ci-op-njttt0ww-initial   ?

SOMETHING UNEXPECTED HERE

= Update Health =
SINCE   LEVEL   IMPACT   MESSAGE
8m57s   Info    None     Update is proceeding well`

func TestMonitor_NoFailures(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		snapshots []snapshot
		expected  *junitapi.JUnitTestCase
	}{
		{
			name: "no snapshots",
			expected: &junitapi.JUnitTestCase{
				Name:        "[sig-cli][OCPFeatureGate:UpgradeStatus] oc amd upgrade status never fails",
				SkipMessage: &junitapi.SkipMessage{Message: "Test skipped because no oc adm upgrade status output was collected"},
			},
		},
		{
			name: "no successful snapshots",
			snapshots: []snapshot{
				{when: time.Now(), err: fmt.Errorf("some error")},
				{when: time.Now(), err: fmt.Errorf("some error")},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc amd upgrade status never fails",
				FailureOutput: &junitapi.FailureOutput{
					Message: "oc adm upgrade status failed 2 times (of 2)",
				},
			},
		},
		{
			name: "two successful snapshots",
			snapshots: []snapshot{
				{when: time.Now(), err: nil, out: exampleOutput},
				{when: time.Now(), err: nil, out: exampleOutput},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc amd upgrade status never fails",
			},
		},
		{
			name: "mixed snapshots",
			snapshots: []snapshot{
				{when: time.Now(), err: nil, out: exampleOutput},
				{when: time.Now(), err: fmt.Errorf("some error")},
				{when: time.Now(), err: nil, out: ""},
				{when: time.Now(), err: nil, out: exampleOutput},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc amd upgrade status never fails",
				FailureOutput: &junitapi.FailureOutput{
					Message: "oc adm upgrade status failed 1 times (of 4)",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := NewOcAdmUpgradeStatusChecker().(*monitor)
			m.ocAdmUpgradeStatus = append(m.ocAdmUpgradeStatus, tc.snapshots...)

			ignoreOutput := cmpopts.IgnoreFields(junitapi.FailureOutput{}, "Output")

			result := m.noFailures()
			if diff := cmp.Diff(tc.expected, result, ignoreOutput); diff != "" {
				t.Errorf("unexpected result (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMonitor_ExpectedLayout(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		snapshots []snapshot
		expected  *junitapi.JUnitTestCase
	}{
		{
			name: "no snapshots",
			expected: &junitapi.JUnitTestCase{
				Name:        "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status output has expected layout",
				SkipMessage: &junitapi.SkipMessage{Message: "Test skipped because no oc adm upgrade status output was successfully collected"},
			},
		},
		{
			name: "two unsuccessful snapshots",
			snapshots: []snapshot{
				{when: time.Now(), err: fmt.Errorf("some error")},
				{when: time.Now(), err: fmt.Errorf("another error")},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status output has expected layout",
				SkipMessage: &junitapi.SkipMessage{
					Message: "Test skipped because no oc adm upgrade status output was successfully collected",
				},
			},
		},
		{
			name: "two successful snapshots",
			snapshots: []snapshot{
				{when: time.Now(), err: nil, out: exampleOutput},
				{when: time.Now(), err: nil, out: exampleOutput},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status output has expected layout",
			},
		},
		{
			name: "errored snapshots do not count",
			snapshots: []snapshot{
				{when: time.Now(), err: nil, out: exampleOutput},
				{when: time.Now(), err: fmt.Errorf("some error")},
				{when: time.Now(), err: nil, out: exampleOutput},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status output has expected layout",
			},
		},
		{
			name: "no error but empty output fails the check",
			snapshots: []snapshot{
				{when: time.Now(), err: nil, out: exampleOutput},
				{when: time.Now(), err: nil, out: ""},
				{when: time.Now(), err: nil, out: exampleOutput},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status output has expected layout",
				FailureOutput: &junitapi.FailureOutput{
					Message: "observed unexpected outputs in oc adm upgrade status",
				},
			},
		},
		{
			name: "nonconforming output fails the check",
			snapshots: []snapshot{
				{when: time.Now(), err: nil, out: exampleOutput},
				{when: time.Now(), err: nil, out: badOutput},
				{when: time.Now(), err: nil, out: exampleOutput},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status output has expected layout",
				FailureOutput: &junitapi.FailureOutput{
					Message: "observed unexpected outputs in oc adm upgrade status",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := NewOcAdmUpgradeStatusChecker().(*monitor)
			m.ocAdmUpgradeStatus = append(m.ocAdmUpgradeStatus, tc.snapshots...)

			ignoreOutput := cmpopts.IgnoreFields(junitapi.FailureOutput{}, "Output")

			result := m.expectedLayout()
			if diff := cmp.Diff(tc.expected, result, ignoreOutput); diff != "" {
				t.Errorf("unexpected result (-want +got):\n%s", diff)
			}
		})
	}
}
