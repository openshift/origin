package admupgradestatus

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var (
	emptyLine          = ""
	clusterNotUpdating = `The cluster is not updating.`

	noTokenNoAlerts = `Unable to fetch alerts, ignoring alerts in 'Update Health':  failed to get alerts from Thanos: no token is currently in use for this session`

	controlPlaneHeader = `= Control Plane =`

	genericControlPlane = `Update to 4.16.0-ec.3 successfully completed at 2024-02-27T15:42:58Z (duration: 3h31m)

All control plane nodes successfully updated to 4.16.0-ec.3`

	controlPlaneSummary = `Assessment:      Stalled
Target Version:  4.14.1 (from 4.14.0-rc.3)
Completion:      97% (32 operators updated, 1 updating, 0 waiting)
Duration:        1h59m (Est. Time Remaining: N/A; estimate duration was 1h24m)
Operator Health: 28 Healthy, 1 Unavailable, 4 Available but degraded`

	controlPlaneSummaryWithUpdating = `Assessment:      Stalled
Target Version:  4.14.1 (from 4.14.0-rc.3)
Updating:        machine-config
Completion:      97% (32 operators updated, 1 updating, 0 waiting)
Duration:        1h59m (Est. Time Remaining: N/A; estimate duration was 1h24m)
Operator Health: 28 Healthy, 1 Unavailable, 4 Available but degraded`

	controlPlaneSummaryInconsistentOperators = `Assessment:      Progressing
Target Version:  4.20.0-0.ci-2025-08-13-182454-test-ci-op-5wilvz46-latest (from 4.20.0-0.ci-2025-08-13-174821-test-ci-op-5wilvz46-initial)
Updating:        image-registry, monitoring, openshift-controller-manager
Completion:      50% (17 operators updated, 3 updating, 14 waiting)
Duration:        24m (Est. Time Remaining: 45m)
Operator Health: 34 Healthy`

	controlPlaneUpdated = `Update to 4.16.0-ec.3 successfully completed at 2024-02-27T15:42:58Z (duration: 3h31m)`

	expectedControlPlaneSummaries = map[string]map[string]string{
		controlPlaneSummary: {
			"Assessment":      "Stalled",
			"Target Version":  "4.14.1 (from 4.14.0-rc.3)",
			"Completion":      "97% (32 operators updated, 1 updating, 0 waiting)",
			"Duration":        "1h59m (Est. Time Remaining: N/A; estimate duration was 1h24m)",
			"Operator Health": "28 Healthy, 1 Unavailable, 4 Available but degraded",
		},
		controlPlaneSummaryWithUpdating: {
			"Assessment":      "Stalled",
			"Target Version":  "4.14.1 (from 4.14.0-rc.3)",
			"Updating":        "machine-config",
			"Completion":      "97% (32 operators updated, 1 updating, 0 waiting)",
			"Duration":        "1h59m (Est. Time Remaining: N/A; estimate duration was 1h24m)",
			"Operator Health": "28 Healthy, 1 Unavailable, 4 Available but degraded",
		},
		controlPlaneSummaryInconsistentOperators: {
			"Assessment":      "Progressing",
			"Target Version":  "4.20.0-0.ci-2025-08-13-182454-test-ci-op-5wilvz46-latest (from 4.20.0-0.ci-2025-08-13-174821-test-ci-op-5wilvz46-initial)",
			"Updating":        "image-registry, monitoring, openshift-controller-manager",
			"Completion":      "50% (17 operators updated, 3 updating, 14 waiting)",
			"Duration":        "24m (Est. Time Remaining: 45m)",
			"Operator Health": "34 Healthy",
		},
		controlPlaneUpdated: nil, // No summary for updated control plane
	}

	controlPlaneOperators = `Updating Cluster Operators
NAME             SINCE     REASON   MESSAGE
machine-config   1h4m41s   -        Working towards 4.14.1`

	// TODO: This is actually a bug we should fix in the output, we will fix this
	controlPlaneInconsistentOperators = `Updating Cluster Operators
NAME             SINCE   REASON                                            MESSAGE
image-registry   6s      DeploymentNotCompleted::NodeCADaemonUnavailable   NodeCADaemonProgressing: The daemon set node-ca is deploying node pods
Progressing: The deployment has not completed
monitoring                     4s    RollOutInProgress                                                                Rolling out the stack.
openshift-controller-manager   11s   RouteControllerManager_DesiredStateNotYetAchieved::_DesiredStateNotYetAchieved   Progressing: deployment/controller-manager: observed generation is 10, desired generation is 11
Progressing: deployment/controller-manager: updated replicas is 1, desired replicas is 3
RouteControllerManagerProgressing: deployment/route-controller-manager: observed generation is 7, desired generation is 8
RouteControllerManagerProgressing: deployment/route-controller-manager: updated replicas is 1, desired replicas is 3`

	expectedControlPlaneOperators = map[string][]string{
		controlPlaneOperators: {"machine-config   1h4m41s   -        Working towards 4.14.1"},
		controlPlaneInconsistentOperators: {
			"image-registry   6s      DeploymentNotCompleted::NodeCADaemonUnavailable   NodeCADaemonProgressing: The daemon set node-ca is deploying node pods",
			"Progressing: The deployment has not completed",
			"monitoring                     4s    RollOutInProgress                                                                Rolling out the stack.",
			"openshift-controller-manager   11s   RouteControllerManager_DesiredStateNotYetAchieved::_DesiredStateNotYetAchieved   Progressing: deployment/controller-manager: observed generation is 10, desired generation is 11",
			"Progressing: deployment/controller-manager: updated replicas is 1, desired replicas is 3",
			"RouteControllerManagerProgressing: deployment/route-controller-manager: observed generation is 7, desired generation is 8",
			"RouteControllerManagerProgressing: deployment/route-controller-manager: updated replicas is 1, desired replicas is 3",
		},
	}

	controlPlaneThreeNodes = `Control Plane Nodes
NAME                                        ASSESSMENT   PHASE     VERSION       EST   MESSAGE
ip-10-0-30-217.us-east-2.compute.internal   Outdated     Pending   4.14.0-rc.3   ?     
ip-10-0-53-40.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?     
ip-10-0-92-180.us-east-2.compute.internal   Outdated     Pending   4.14.0-rc.3   ?     `

	controlPlaneNodesUpdated = `All control plane nodes successfully updated to 4.16.0-ec.3`

	expectedControlPlaneNodes = map[string][]string{
		controlPlaneThreeNodes: {
			"ip-10-0-30-217.us-east-2.compute.internal   Outdated     Pending   4.14.0-rc.3   ?",
			"ip-10-0-53-40.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?",
			"ip-10-0-92-180.us-east-2.compute.internal   Outdated     Pending   4.14.0-rc.3   ?",
		},
		controlPlaneNodesUpdated: nil,
	}

	workerSectionHeader = `= Worker Upgrade =`

	genericWorkerPool  = oneWorkerPool
	genericWorkerNodes = oneWorkerPoolNodes

	oneWorkerPool = `WORKER POOL   ASSESSMENT   COMPLETION   STATUS
worker        Pending      0% (0/3)     3 Available, 0 Progressing, 0 Draining`

	twoPools = `WORKER POOL   ASSESSMENT    COMPLETION   STATUS
worker        Progressing   0% (0/2)     1 Available, 1 Progressing, 1 Draining
infra         Progressing   0% (0/2)     1 Available, 1 Progressing, 1 Draining`

	twoPoolsOneEmpty = `WORKER POOL   ASSESSMENT   COMPLETION   STATUS
worker        Pending      0% (0/3)     3 Available, 0 Progressing, 0 Draining
zbeast        Empty                     0 Total`

	expectedPools = map[string][]string{
		oneWorkerPool: {"worker        Pending      0% (0/3)     3 Available, 0 Progressing, 0 Draining"},
		twoPools: {
			"worker        Progressing   0% (0/2)     1 Available, 1 Progressing, 1 Draining",
			"infra         Progressing   0% (0/2)     1 Available, 1 Progressing, 1 Draining",
		},
		twoPoolsOneEmpty: {
			"worker        Pending      0% (0/3)     3 Available, 0 Progressing, 0 Draining",
			"zbeast        Empty                     0 Total",
		},
	}

	oneWorkerPoolNodes = `Worker Pool Nodes: worker
NAME                                        ASSESSMENT   PHASE     VERSION       EST   MESSAGE
ip-10-0-20-162.us-east-2.compute.internal   Outdated     Pending   4.14.0-rc.3   ?     
ip-10-0-4-159.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?     
ip-10-0-99-40.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?     `

	twoPoolsWorkerNodes = `Worker Pool Nodes: worker
NAME                                       ASSESSMENT    PHASE      VERSION   EST    MESSAGE
ip-10-0-4-159.us-east-2.compute.internal   Progressing   Draining   4.14.0    +10m   
ip-10-0-99-40.us-east-2.compute.internal   Outdated      Pending    4.14.0    ?      `

	twoPoolsInfraNodes = `Worker Pool Nodes: infra
NAME                                             ASSESSMENT    PHASE      VERSION   EST    MESSAGE
ip-10-0-4-159-infra.us-east-2.compute.internal   Progressing   Draining   4.14.0    +10m   
ip-10-0-20-162.us-east-2.compute.internal        Outdated      Pending    4.14.0    ?      `

	expectedPoolNodes = map[string]map[string][]string{
		oneWorkerPool: {
			"worker": {
				"ip-10-0-20-162.us-east-2.compute.internal   Outdated     Pending   4.14.0-rc.3   ?",
				"ip-10-0-4-159.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?",
				"ip-10-0-99-40.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?",
			},
		},
		twoPools: {
			"worker": {
				"ip-10-0-4-159.us-east-2.compute.internal   Progressing   Draining   4.14.0    +10m",
				"ip-10-0-99-40.us-east-2.compute.internal   Outdated      Pending    4.14.0    ?",
			},
			"infra": {
				"ip-10-0-4-159-infra.us-east-2.compute.internal   Progressing   Draining   4.14.0    +10m",
				"ip-10-0-20-162.us-east-2.compute.internal        Outdated      Pending    4.14.0    ?",
			},
		},
		twoPoolsOneEmpty: {
			"worker": {
				"ip-10-0-20-162.us-east-2.compute.internal   Outdated     Pending   4.14.0-rc.3   ?",
				"ip-10-0-4-159.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?",
				"ip-10-0-99-40.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?",
			},
		},
	}

	healthSectionHeader = `= Update Health = `

	genericHealthSection = healthProceedingWell

	healthProceedingWell = `SINCE    LEVEL   IMPACT   MESSAGE
52m56s   Info    None     Update is proceeding well`

	healthMultipleTable = `SINCE     LEVEL     IMPACT             MESSAGE
58m18s    Error     API Availability   Cluster Operator kube-apiserver is degraded (NodeController_MasterNodesReady)
now       Warning   Update Stalled     Cluster Version version is failing to proceed with the update (ClusterOperatorsDegraded)`

	healthDetailed = `Message: Cluster Operator kube-apiserver is degraded (NodeController_MasterNodesReady)
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

	expectedHealthMessages = map[string][]string{
		healthProceedingWell: {"52m56s   Info    None     Update is proceeding well"},
		healthMultipleTable: {
			"58m18s    Error     API Availability   Cluster Operator kube-apiserver is degraded (NodeController_MasterNodesReady)",
			"now       Warning   Update Stalled     Cluster Version version is failing to proceed with the update (ClusterOperatorsDegraded)",
		},
		healthDetailed: {
			`Message: Cluster Operator kube-apiserver is degraded (NodeController_MasterNodesReady)
  Since:       58m18s
  Level:       Error
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusteroperators.config.openshift.io: kube-apiserver
  Description: NodeControllerDegraded: The master nodes not ready: node "ip-10-0-12-74.ec2.internal" not ready since 2023-11-03 16:28:43 +0000 UTC because KubeletNotReady (container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?)`,
			`Message: Cluster Version version is failing to proceed with the update (ClusterOperatorsDegraded)
  Since:       now
  Level:       Warning
  Impact:      Update Stalled
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusterversions.config.openshift.io: version
  Description: Cluster operators etcd, kube-apiserver are degraded`,
		},
	}
)

func TestUpgradeStatusOutput_Updating(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		segments []string
		expected bool
	}{
		{
			name:     "cluster not updating",
			segments: []string{clusterNotUpdating},
			expected: false,
		},
		{
			name: "cluster updating",
			segments: []string{
				controlPlaneHeader,
				genericControlPlane,
				emptyLine,
				workerSectionHeader,
				genericWorkerPool,
				emptyLine,
				genericWorkerNodes,
				emptyLine,
				healthSectionHeader,
				healthProceedingWell,
			},
			expected: true,
		},
		{
			name: "cluster updating | no token no alerts",
			segments: []string{
				noTokenNoAlerts,
				controlPlaneHeader,
				genericControlPlane,
				emptyLine,
				workerSectionHeader,
				genericWorkerPool,
				emptyLine,
				genericWorkerNodes,
				emptyLine,
				healthSectionHeader,
				healthProceedingWell,
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			builder := strings.Builder{}
			for _, input := range tc.segments {
				builder.WriteString(input)
				builder.WriteString("\n")
			}

			output, err := newUpgradeStatusOutput(builder.String())
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if output.updating != tc.expected {
				t.Errorf("Expected IsUpdating() to return %v, got %v", tc.expected, output.updating)
			}
		})
	}
}

func TestUpgradeStatusOutput_ControlPlane(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		segments    []string
		expected    *ControlPlaneStatus
		expectError string
	}{
		{
			name:     "cluster not updating",
			segments: []string{clusterNotUpdating},
			expected: nil,
		},
		{
			name: "control plane without updating operators line",
			segments: []string{
				controlPlaneHeader,
				controlPlaneSummary,
				emptyLine,
				controlPlaneThreeNodes,
				emptyLine,
				healthSectionHeader,
				genericHealthSection,
			},
			expected: &ControlPlaneStatus{
				Updated:      false,
				Summary:      expectedControlPlaneSummaries[controlPlaneSummary],
				Operators:    nil, // No operators line present
				NodesUpdated: false,
				Nodes:        expectedControlPlaneNodes[controlPlaneThreeNodes],
			},
		},
		{
			name: "control plane without updating operators line, no token warning",
			segments: []string{
				noTokenNoAlerts,
				controlPlaneHeader,
				controlPlaneSummary,
				emptyLine,
				controlPlaneThreeNodes,
				emptyLine,
				healthSectionHeader,
				genericHealthSection,
			},
			expected: &ControlPlaneStatus{
				Updated:      false,
				Summary:      expectedControlPlaneSummaries[controlPlaneSummary],
				Operators:    nil, // No operators line present
				NodesUpdated: false,
				Nodes:        expectedControlPlaneNodes[controlPlaneThreeNodes],
			},
		},
		{
			name: "control plane with updating",
			segments: []string{
				controlPlaneHeader,
				controlPlaneSummaryWithUpdating,
				emptyLine,
				controlPlaneOperators,
				emptyLine,
				controlPlaneThreeNodes,
				emptyLine,
				healthSectionHeader,
				genericHealthSection,
			},
			expected: &ControlPlaneStatus{
				Updated:      false,
				Summary:      expectedControlPlaneSummaries[controlPlaneSummaryWithUpdating],
				Operators:    expectedControlPlaneOperators[controlPlaneOperators],
				NodesUpdated: false,
				Nodes:        expectedControlPlaneNodes[controlPlaneThreeNodes],
			},
		},
		{
			name: "control plane with updated nodes",
			segments: []string{
				controlPlaneHeader,
				controlPlaneSummary,
				emptyLine,
				controlPlaneNodesUpdated,
				emptyLine,
				healthSectionHeader,
				genericHealthSection,
			},
			expected: &ControlPlaneStatus{
				Updated:      false,
				Summary:      expectedControlPlaneSummaries[controlPlaneSummary],
				Operators:    nil,
				NodesUpdated: true,
				Nodes:        nil,
			},
		},
		{
			name: "updated control plane",
			segments: []string{
				controlPlaneHeader,
				controlPlaneUpdated,
				emptyLine,
				controlPlaneNodesUpdated,
				emptyLine,
				healthSectionHeader,
				genericHealthSection,
			},
			expected: &ControlPlaneStatus{
				Updated:      true,
				Summary:      nil,
				Operators:    nil,
				NodesUpdated: true,
				Nodes:        nil,
			},
		},
		{
			name: "control plane with inconsistent operators (bug that will be fixed)",
			segments: []string{
				controlPlaneHeader,
				controlPlaneSummaryInconsistentOperators,
				emptyLine,
				controlPlaneInconsistentOperators,
				emptyLine,
				controlPlaneThreeNodes,
				emptyLine,
				healthSectionHeader,
				genericHealthSection,
			},
			expected: &ControlPlaneStatus{
				Updated:      false,
				Summary:      expectedControlPlaneSummaries[controlPlaneSummaryInconsistentOperators],
				Operators:    expectedControlPlaneOperators[controlPlaneInconsistentOperators],
				NodesUpdated: false,
				Nodes:        expectedControlPlaneNodes[controlPlaneThreeNodes],
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			builder := strings.Builder{}
			for _, input := range tc.segments {
				builder.WriteString(input)
				builder.WriteString("\n")
			}

			output, err := newUpgradeStatusOutput(builder.String())
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if diff := cmp.Diff(tc.expected, output.controlPlane); diff != "" {
				t.Errorf("ControlPlane mismatch (-expected +actual):\n%s", diff)
			}
		})
	}
}

func TestUpgradeStatusOutput_Workers(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		segments []string
		expected *WorkersStatus
	}{
		{
			name:     "cluster not updating",
			segments: []string{clusterNotUpdating},
			expected: nil,
		},
		{
			name: "worker section is optional (SNO & compact)",
			segments: []string{
				controlPlaneHeader,
				genericControlPlane,
				emptyLine,
				healthSectionHeader,
				genericHealthSection,
			},
			expected: nil,
		},
		{
			name: "one pool with three nodes",
			segments: []string{
				controlPlaneHeader,
				genericControlPlane,
				emptyLine,
				workerSectionHeader,
				emptyLine,
				oneWorkerPool,
				emptyLine,
				oneWorkerPoolNodes,
				emptyLine,
				healthSectionHeader,
				genericHealthSection,
			},
			expected: &WorkersStatus{
				Pools: expectedPools[oneWorkerPool],
				Nodes: expectedPoolNodes[oneWorkerPool],
			},
		},
		{
			name: "two pools with two nodes each",
			segments: []string{
				controlPlaneHeader,
				genericControlPlane,
				emptyLine,
				workerSectionHeader,
				emptyLine,
				twoPools,
				emptyLine,
				twoPoolsWorkerNodes,
				emptyLine,
				twoPoolsInfraNodes,
				emptyLine,
				healthSectionHeader,
				genericHealthSection,
			},
			expected: &WorkersStatus{
				Pools: expectedPools[twoPools],
				Nodes: expectedPoolNodes[twoPools],
			},
		},
		{
			name: "two pools, one of them empty",
			segments: []string{
				controlPlaneHeader,
				genericControlPlane,
				emptyLine,
				workerSectionHeader,
				emptyLine,
				twoPoolsOneEmpty,
				emptyLine,
				oneWorkerPoolNodes,
				emptyLine,
				healthSectionHeader,
				genericHealthSection,
			},
			expected: &WorkersStatus{
				Pools: expectedPools[twoPoolsOneEmpty],
				Nodes: expectedPoolNodes[twoPoolsOneEmpty],
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			builder := strings.Builder{}
			for _, input := range tc.segments {
				builder.WriteString(input)
				builder.WriteString("\n")
			}

			output, err := newUpgradeStatusOutput(builder.String())
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if diff := cmp.Diff(tc.expected, output.workers); diff != "" {
				t.Errorf("Workers mismatch (-expected +actual):\n%s", diff)
			}
		})
	}
}

func TestUpgradeStatusOutput_Health(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		segments []string
		expected *Health
	}{
		{
			name:     "cluster not updating",
			segments: []string{clusterNotUpdating},
			expected: nil,
		},
		{
			name: "Update is proceeding well",
			segments: []string{
				controlPlaneHeader,
				genericControlPlane,
				emptyLine,
				workerSectionHeader,
				genericWorkerPool,
				emptyLine,
				genericWorkerNodes,
				emptyLine,
				healthSectionHeader,
				healthProceedingWell,
			},
			expected: &Health{
				Detailed: false,
				Messages: expectedHealthMessages[healthProceedingWell],
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			builder := strings.Builder{}
			for _, input := range tc.segments {
				builder.WriteString(input)
				builder.WriteString("\n")
			}

			output, err := newUpgradeStatusOutput(builder.String())
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if diff := cmp.Diff(tc.expected, output.health); diff != "" {
				t.Errorf("Health mismatch (-expected +actual):\n%s", diff)
			}
		})
	}
}
