package admupgradestatus

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

var (
	lifecycle01before = `The cluster is not updating.`

	lifecycle02updating = `Unable to fetch alerts, ignoring alerts in 'Update Health':  failed to get alerts from Thanos: no token is currently in use for this session
= Control Plane =
Assessment:      Progressing
Target Version:  4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest (from 4.20.0-0.ci-2025-08-13-114210-test-ci-op-njttt0ww-initial)
Updating:        etcd, kube-apiserver
Completion:      3% (1 operators updated, 2 updating, 31 waiting)
Duration:        3m51s (Est. Time Remaining: 1h7m)
Operator Health: 34 Healthy

Updating Cluster Operators
NAME             SINCE   REASON          MESSAGE
etcd             2m53s   NodeInstaller   NodeInstallerProgressing: 2 nodes are at revision 8; 1 node is at revision 9
kube-apiserver   2m21s   NodeInstaller   NodeInstallerProgressing: 3 nodes are at revision 7; 0 nodes have achieved new revision 8

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
3m51s   Info    None     Update is proceeding well`

	lifecycle03controlPlaneNodesUpdated = `Unable to fetch alerts, ignoring alerts in 'Update Health':  failed to get alerts from Thanos: no token is currently in use for this session
= Control Plane =
Assessment:      Progressing - Slow
Target Version:  4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest (from 4.20.0-0.ci-2025-08-13-114210-test-ci-op-njttt0ww-initial)
Updating:        machine-config
Completion:      97% (33 operators updated, 1 updating, 0 waiting)
Duration:        1h1m (Est. Time Remaining: <10m)
Operator Health: 30 Healthy, 4 Available but degraded

Updating Cluster Operators
NAME             SINCE    REASON   MESSAGE
machine-config   22m21s   -        Working towards 4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest

All control plane nodes successfully updated to 4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest

= Worker Upgrade =

WORKER POOL   ASSESSMENT   COMPLETION   STATUS
worker        Completed    100% (3/3)   3 Available, 0 Progressing, 0 Draining

Worker Pool Nodes: worker
NAME                                         ASSESSMENT   PHASE     VERSION                                                    EST   MESSAGE
ip-10-0-0-72.us-west-1.compute.internal      Completed    Updated   4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest   -     
ip-10-0-100-255.us-west-1.compute.internal   Completed    Updated   4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest   -     
ip-10-0-106-212.us-west-1.compute.internal   Completed    Updated   4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest   -     

= Update Health =
SINCE     LEVEL   IMPACT   MESSAGE
1h1m36s   Info    None     Update is proceeding well`

	lifecycle04controlPlaneUpdated = `Unable to fetch alerts, ignoring alerts in 'Update Health':  failed to get alerts from Thanos: no token is currently in use for this session
= Control Plane =
Update to 4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest successfully completed at 2025-08-13T14:15:18Z (duration: 1h2m)

All control plane nodes successfully updated to 4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest

= Worker Upgrade =

WORKER POOL   ASSESSMENT   COMPLETION   STATUS
worker        Completed    100% (3/3)   3 Available, 0 Progressing, 0 Draining

Worker Pool Nodes: worker
NAME                                         ASSESSMENT   PHASE     VERSION                                                    EST   MESSAGE
ip-10-0-0-72.us-west-1.compute.internal      Completed    Updated   4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest   -     
ip-10-0-100-255.us-west-1.compute.internal   Completed    Updated   4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest   -     
ip-10-0-106-212.us-west-1.compute.internal   Completed    Updated   4.20.0-0.ci-2025-08-13-121604-test-ci-op-njttt0ww-latest   -     

= Update Health =
SINCE     LEVEL   IMPACT   MESSAGE
1h1m36s   Info    None     Update is proceeding well`

	lifecycle05after = `The cluster is not updating.`
)

func TestMonitor_UpdateLifecycle(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		snapshots  []snapshot
		wasUpdated bool
		expected   *junitapi.JUnitTestCase
	}{
		{
			name: "no snapshots -> test skipped",
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status snapshots reflect the cluster upgrade lifecycle",
				SkipMessage: &junitapi.SkipMessage{
					Message: "Test skipped because no oc adm upgrade status output was successfully collected",
				},
			},
		},
		{
			name: "model update",
			snapshots: []snapshot{
				{when: time.Now(), out: lifecycle01before},
				{when: time.Now(), out: lifecycle02updating},
				{when: time.Now(), out: lifecycle03controlPlaneNodesUpdated},
				{when: time.Now(), out: lifecycle04controlPlaneUpdated},
				{when: time.Now(), out: lifecycle05after},
			},
			wasUpdated: true,
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status snapshots reflect the cluster upgrade lifecycle",
			},
		},
		{
			name: "sometimes we miss the control plane updated state, this is okay",
			snapshots: []snapshot{
				{when: time.Now(), out: lifecycle01before},
				{when: time.Now(), out: lifecycle02updating},
				{when: time.Now(), out: lifecycle05after},
			},
			wasUpdated: true,
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status snapshots reflect the cluster upgrade lifecycle",
			},
		},
		{
			name: "completed control plane nodes went back to updating",
			snapshots: []snapshot{
				{when: time.Now(), out: lifecycle01before},
				{when: time.Now(), out: lifecycle02updating},
				{when: time.Now(), out: lifecycle03controlPlaneNodesUpdated},
				{when: time.Now(), out: lifecycle02updating},
				{when: time.Now(), out: lifecycle05after},
			},
			wasUpdated: true,
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status snapshots reflect the cluster upgrade lifecycle",
				FailureOutput: &junitapi.FailureOutput{
					Message: "observed unexpected update lifecycle transition in oc adm upgrade status",
				},
			},
		},
		{
			name: "no update observed when cluster was not updated",
			snapshots: []snapshot{
				{when: time.Now(), out: lifecycle01before},
				{when: time.Now(), out: lifecycle01before},
				{when: time.Now(), out: lifecycle01before},
			},
			wasUpdated: false,
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status snapshots reflect the cluster upgrade lifecycle",
			},
		},
		{
			name: "update observed when cluster was not updated",
			snapshots: []snapshot{
				{when: time.Now(), out: lifecycle01before},
				{when: time.Now(), out: lifecycle02updating},
				{when: time.Now(), out: lifecycle01before},
			},
			wasUpdated: false,
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status snapshots reflect the cluster upgrade lifecycle",
				FailureOutput: &junitapi.FailureOutput{
					Message: "observed unexpected update lifecycle transition in oc adm upgrade status",
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

			// Process snapshots into models for the health check to work with
			_ = m.expectedLayout()

			wasUpdated := func() (bool, error) {
				return tc.wasUpdated, nil
			}

			result := m.updateLifecycle(wasUpdated)
			if diff := cmp.Diff(tc.expected, result, ignoreOutput); diff != "" {
				t.Errorf("unexpected result (-want +got):\n%s", diff)
			}
		})
	}
}
