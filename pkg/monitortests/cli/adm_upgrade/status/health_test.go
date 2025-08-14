package admupgradestatus

import (
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

var healthExampleOutput = `Unable to fetch alerts, ignoring alerts in 'Update Health':  failed to get alerts from Thanos: no token is currently in use for this session
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

var healthBadOutput = `= Control Plane =
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

var healthTableOutput = `Unable to fetch alerts, ignoring alerts in 'Update Health':  failed to get alerts from Thanos: no token is currently in use for this session
= Control Plane =
Update to 4.16.0-ec.3 successfully completed at 2024-02-27T15:42:58Z (duration: 3h31m)

All control plane nodes successfully updated to 4.16.0-ec.3

= Update Health =
SINCE     LEVEL     IMPACT             MESSAGE
58m18s    Error     API Availability   Cluster Operator kube-apiserver is degraded (NodeController_MasterNodesReady)
now       Warning   Update Stalled     Cluster Version version is failing to proceed with the update (ClusterOperatorsDegraded)`

var healthDetailedOutputSingle = `Unable to fetch alerts, ignoring alerts in 'Update Health':  failed to get alerts from Thanos: no token is currently in use for this session
= Control Plane =
Update to 4.16.0-ec.3 successfully completed at 2024-02-27T15:42:58Z (duration: 3h31m)

All control plane nodes successfully updated to 4.16.0-ec.3

= Update Health =
Message: Cluster Operator kube-apiserver is degraded (NodeController_MasterNodesReady)
  Since:       58m18s
  Level:       Error
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusteroperators.config.openshift.io: kube-apiserver
  Description: NodeControllerDegraded: The master nodes not ready: node "ip-10-0-12-74.ec2.internal" not ready since 2023-11-03 16:28:43 +0000 UTC because KubeletNotReady (container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?)`

var healthDetailedOutputMultiple = `
Unable to fetch alerts, ignoring alerts in 'Update Health':  failed to get alerts from Thanos: no token is currently in use for this session
= Control Plane =
Update to 4.16.0-ec.3 successfully completed at 2024-02-27T15:42:58Z (duration: 3h31m)

All control plane nodes successfully updated to 4.16.0-ec.3

= Update Health =
Message: Cluster Operator kube-apiserver is degraded (NodeController_MasterNodesReady)
  Since:       58m18s
  Level:       Error
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusteroperators.config.openshift.io: kube-apiserver
  Description: NodeControllerDegraded: The master nodes not ready: node "ip-10-0-12-74.ec2.internal" not ready since 2023-11-03 16:28:43 +0000 UTC because KubeletNotReady (container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?)

Message: Cluster Version version is failing to proceed with the update (ClusterOperatorsDegraded)
  Since:       now
  Level:       Warning
  Impact:      Update Stalled
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusterversions.config.openshift.io: version
  Description: Cluster operators etcd, kube-apiserver are degraded`

var healthMissingField = `Unable to fetch alerts, ignoring alerts in 'Update Health':  failed to get alerts from Thanos: no token is currently in use for this session
= Control Plane =
Update to 4.16.0-ec.3 successfully completed at 2024-02-27T15:42:58Z (duration: 3h31m)

All control plane nodes successfully updated to 4.16.0-ec.3

= Update Health =
Message: Cluster Operator kube-apiserver is degraded (NodeController_MasterNodesReady)
  Since:       58m18s
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusteroperators.config.openshift.io: kube-apiserver
  Description: NodeControllerDegraded: The master nodes not ready: node "ip-10-0-12-74.ec2.internal" not ready since 2023-11-03 16:28:43 +0000 UTC because KubeletNotReady (container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?)`

var healthEmptyField = `Unable to fetch alerts, ignoring alerts in 'Update Health':  failed to get alerts from Thanos: no token is currently in use for this session
= Control Plane =
Update to 4.16.0-ec.3 successfully completed at 2024-02-27T15:42:58Z (duration: 3h31m)

All control plane nodes successfully updated to 4.16.0-ec.3

= Update Health =
Message: Cluster Operator kube-apiserver is degraded (NodeController_MasterNodesReady)
  Since:       
  Level:       Warning
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusteroperators.config.openshift.io: kube-apiserver
  Description: NodeControllerDegraded: The master nodes not ready: node "ip-10-0-12-74.ec2.internal" not ready since 2023-11-03 16:28:43 +0000 UTC because KubeletNotReady (container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?)`

func TestMonitor_Health(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		snapshots []snapshot
		expected  *junitapi.JUnitTestCase
	}{
		{
			name: "no snapshots -> test skipped",
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status health section is consistent",
				SkipMessage: &junitapi.SkipMessage{
					Message: "Test skipped because no oc adm upgrade status output was successfully collected",
				},
			},
		},
		{
			name: "good snapshots",
			snapshots: []snapshot{
				{when: time.Now(), out: healthExampleOutput},
				{when: time.Now(), out: healthExampleOutput},
				{when: time.Now(), out: healthExampleOutput},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status health section is consistent",
			},
		},
		{
			name: "errored snapshots are skipped",
			snapshots: []snapshot{
				{when: time.Now(), out: healthExampleOutput},
				{when: time.Now(), out: healthBadOutput, err: errors.New("some error")},
				{when: time.Now(), out: healthExampleOutput},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status health section is consistent",
			},
		},
		{
			name: "unparseable snapshots are skipped",
			snapshots: []snapshot{
				{when: time.Now(), out: healthExampleOutput},
				{when: time.Now(), out: "unparseable output"},
				{when: time.Now(), out: healthExampleOutput},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status health section is consistent",
			},
		},
		{
			name: "multiple table lines",
			snapshots: []snapshot{
				{when: time.Now(), out: healthTableOutput},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status health section is consistent",
			},
		},
		{
			name: "detailed output single item",
			snapshots: []snapshot{
				{when: time.Now(), out: healthDetailedOutputSingle},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status health section is consistent",
			},
		},
		{
			name: "detailed output multiple items",
			snapshots: []snapshot{
				{when: time.Now(), out: healthDetailedOutputMultiple},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status health section is consistent",
			},
		},
		{
			name: "missing item from detailed output",
			snapshots: []snapshot{
				{when: time.Now(), out: healthMissingField},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status health section is consistent",
				FailureOutput: &junitapi.FailureOutput{
					Message: "observed unexpected outputs in oc adm upgrade status health section",
				},
			},
		},
		{
			name: "empty item from detailed output",
			snapshots: []snapshot{
				{when: time.Now(), out: healthEmptyField},
			},
			expected: &junitapi.JUnitTestCase{
				Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status health section is consistent",
				FailureOutput: &junitapi.FailureOutput{
					Message: "observed unexpected outputs in oc adm upgrade status health section",
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

			result := m.health()
			if diff := cmp.Diff(tc.expected, result, ignoreOutput); diff != "" {
				t.Errorf("unexpected result (-want +got):\n%s", diff)
			}
		})
	}
}
