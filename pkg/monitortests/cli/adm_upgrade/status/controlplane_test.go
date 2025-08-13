package admupgradestatus

import (
	"errors"
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

var missingItemInSummary = `= Control Plane =
Target Version:  4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest (from 4.20.0-0.ci-2025-08-13-114210-test-ci-op-njttt0ww-initial)
Completion:      6% (2 operators updated, 1 updating, 31 waiting)
Duration:        8m57s (Est. Time Remaining: 1h9m)
Operator Health: 34 Healthy

Control Plane Nodes
NAME                                        ASSESSMENT   PHASE     VERSION                                                     EST   MESSAGE
ip-10-0-111-19.us-west-1.compute.internal   Outdated     Pending   4.20.0-0.ci-2025-08-13-114210-test-ci-op-njttt0ww-initial   ?

= Update Health =
SINCE   LEVEL   IMPACT   MESSAGE
8m57s   Info    None     Update is proceeding well`

var operatorsSectionWithoutSummaryLine = `= Control Plane =
Assessment:      Progressing
Target Version:  4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest (from 4.20.0-0.ci-2025-08-13-114210-test-ci-op-njttt0ww-initial)
Completion:      6% (2 operators updated, 1 updating, 31 waiting)
Duration:        8m57s (Est. Time Remaining: 1h9m)
Operator Health: 34 Healthy

Updating Cluster Operators
NAME             SINCE   REASON          MESSAGE
kube-apiserver   7m27s   NodeInstaller   NodeInstallerProgressing: 1 node is at revision 7; 2 nodes are at revision 8

Control Plane Nodes
NAME                                        ASSESSMENT   PHASE     VERSION                                                     EST   MESSAGE
ip-10-0-111-19.us-west-1.compute.internal   Outdated     Pending   4.20.0-0.ci-2025-08-13-114210-test-ci-op-njttt0ww-initial   ?

= Update Health =
SINCE   LEVEL   IMPACT   MESSAGE
8m57s   Info    None     Update is proceeding well`

func TestMonitor_ControlPlane(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		snapshots []snapshot
		expected  *junitapi.JUnitTestCase
	}{
		{
			name: "no snapshots -> test skipped",
			expected: &junitapi.JUnitTestCase{
				Name:        "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status control plane section is consistent",
				SkipMessage: &junitapi.SkipMessage{Message: "Test skipped because no oc adm upgrade status output was successfully collected"},
			},
		},
		{
			name: "good snapshots",
			snapshots: []snapshot{
				{when: time.Now(), out: exampleOutput},
				{when: time.Now(), out: exampleOutput},
				{when: time.Now(), out: exampleOutput},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status control plane section is consistent",
			},
		},
		{
			name: "errored snapshots are skipped",
			snapshots: []snapshot{
				{when: time.Now(), out: exampleOutput},
				{when: time.Now(), out: badOutput, err: errors.New("some error")},
				{when: time.Now(), out: exampleOutput},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status control plane section is consistent",
			},
		},
		{
			name: "unparseable snapshots are skipped",
			snapshots: []snapshot{
				{when: time.Now(), out: exampleOutput},
				{when: time.Now(), out: "unparseable output"},
				{when: time.Now(), out: exampleOutput},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status control plane section is consistent",
			},
		},
		{
			name: "missing items in summary",
			snapshots: []snapshot{
				{when: time.Now(), out: missingItemInSummary},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status control plane section is consistent",
				FailureOutput: &junitapi.FailureOutput{
					Message: "observed unexpected outputs in oc adm upgrade status control plane section",
				},
			},
		},
		{
			name: "operators section without summary line",
			snapshots: []snapshot{
				{when: time.Now(), out: operatorsSectionWithoutSummaryLine},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status control plane section is consistent",
				FailureOutput: &junitapi.FailureOutput{
					Message: "observed unexpected outputs in oc adm upgrade status control plane section",
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

			// Process snapshots into models for the controlPlane check to work with
			_ = m.expectedLayout()

			result := m.controlPlane()
			if diff := cmp.Diff(tc.expected, result, ignoreOutput); diff != "" {
				t.Errorf("unexpected result (-want +got):\n%s", diff)
			}
		})
	}
}
