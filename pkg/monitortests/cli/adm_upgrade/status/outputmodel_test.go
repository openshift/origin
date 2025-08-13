package admupgradestatus

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUpgradeStatusOutput_NotUpdating(t *testing.T) {
	input := "The cluster is not updating."

	output, err := newUpgradeStatusOutput(input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if output.updating {
		t.Error("Expected IsUpdating() to return false for 'not updating' case")
	}

	if output.controlPlane != nil {
		t.Error("Expected nil ControlPlane() for 'not updating' case")
	}

	if output.workers != nil {
		t.Error("Expected nil Workers() for 'not updating' case")
	}

	if output.health != nil {
		t.Error("Expected nil Health() for 'not updating' case")
	}
}

func TestUpgradeStatusOutput_FullInput(t *testing.T) {
	input := `= Control Plane =
Assessment:      Stalled
Target Version:  4.14.1 (from 4.14.0-rc.3)
Updating:        machine-config
Completion:      97% (32 operators updated, 1 updating, 0 waiting)
Duration:        1h59m (Est. Time Remaining: N/A; estimate duration was 1h24m)
Operator Health: 28 Healthy, 1 Unavailable, 4 Available but degraded

Updating Cluster Operators
NAME             SINCE     REASON   MESSAGE
machine-config   1h4m41s   -        Working towards 4.14.1

Control Plane Nodes
NAME                                        ASSESSMENT   PHASE     VERSION       EST   MESSAGE
ip-10-0-30-217.us-east-2.compute.internal   Outdated     Pending   4.14.0-rc.3   ?     
ip-10-0-53-40.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?     
ip-10-0-92-180.us-east-2.compute.internal   Outdated     Pending   4.14.0-rc.3   ?     

= Worker Upgrade =

WORKER POOL   ASSESSMENT   COMPLETION   STATUS
worker        Pending      0% (0/3)     3 Available, 0 Progressing, 0 Draining

Worker Pool Nodes: worker
NAME                                        ASSESSMENT   PHASE     VERSION       EST   MESSAGE
ip-10-0-20-162.us-east-2.compute.internal   Outdated     Pending   4.14.0-rc.3   ?     
ip-10-0-4-159.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?     
ip-10-0-99-40.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?     

= Update Health =
Message: Cluster Operator kube-apiserver is degraded (NodeController_MasterNodesReady)
  Since:       58m18s
  Level:       Error
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusteroperators.config.openshift.io: kube-apiserver
  Description: NodeControllerDegraded: The master nodes not ready: node "ip-10-0-12-74.ec2.internal" not ready since 2023-11-03 16:28:43 +0000 UTC because KubeletNotReady (container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?)

Message: Cluster Operator kube-controller-manager is degraded (NodeController_MasterNodesReady)
  Since:       58m18s
  Level:       Error
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusteroperators.config.openshift.io: kube-controller-manager
  Description: NodeControllerDegraded: The master nodes not ready: node "ip-10-0-12-74.ec2.internal" not ready since 2023-11-03 16:28:43 +0000 UTC because KubeletNotReady (container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?)

Message: Cluster Operator kube-scheduler is degraded (NodeController_MasterNodesReady)
  Since:       58m18s
  Level:       Error
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusteroperators.config.openshift.io: kube-scheduler
  Description: NodeControllerDegraded: The master nodes not ready: node "ip-10-0-12-74.ec2.internal" not ready since 2023-11-03 16:28:43 +0000 UTC because KubeletNotReady (container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?)

Message: Cluster Operator etcd is degraded (EtcdEndpoints_ErrorUpdatingEtcdEndpoints::EtcdMembers_UnhealthyMembers::NodeController_MasterNodesReady)
  Since:       58m38s
  Level:       Error
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusteroperators.config.openshift.io: etcd
  Description: EtcdEndpointsDegraded: EtcdEndpointsController can't evaluate whether quorum is safe: etcd cluster has quorum of 2 and 2 healthy members which is not fault tolerant: [{Member:ID:12895393557789359222 name:"ip-10-0-73-118.ec2.internal" peerURLs:"https://10.0.73.118:2380" clientURLs:"https://10.0.73.118:2379"  Healthy:true Took:1.725492ms Error:<nil>} {Member:ID:13608765340770574953 name:"ip-10-0-0-60.ec2.internal" peerURLs:"https://10.0.0.60:2380" clientURLs:"https://10.0.0.60:2379"  Healthy:true Took:1.542919ms Error:<nil>} {Member:ID:18044478200504924924 name:"ip-10-0-12-74.ec2.internal" peerURLs:"https://10.0.12.74:2380" clientURLs:"https://10.0.12.74:2379"  Healthy:false Took: Error:create client failure: failed to make etcd client for endpoints [https://10.0.12.74:2379]: context deadline exceeded}]
               , EtcdMembersDegraded: 2 of 3 members are available, ip-10-0-12-74.ec2.internal is unhealthy
               , NodeControllerDegraded: The master nodes not ready: node "ip-10-0-12-74.ec2.internal" not ready since 2023-11-03 16:28:43 +0000 UTC because KubeletNotReady (container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?)

Message: Cluster Operator control-plane-machine-set is unavailable (UnavailableReplicas)
  Since:       1h0m17s
  Level:       Error
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDown.md
  Resources:
    clusteroperators.config.openshift.io: control-plane-machine-set
  Description: Missing 1 available replica(s)

Message: Cluster Version version is failing to proceed with the update (ClusterOperatorsDegraded)
  Since:       now
  Level:       Warning
  Impact:      Update Stalled
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusterversions.config.openshift.io: version
  Description: Cluster operators etcd, kube-apiserver are degraded`

	expectedControlPlaneSummary := map[string]string{
		"Assessment":      "Stalled",
		"Target Version":  "4.14.1 (from 4.14.0-rc.3)",
		"Updating":        "machine-config",
		"Completion":      "97% (32 operators updated, 1 updating, 0 waiting)",
		"Duration":        "1h59m (Est. Time Remaining: N/A; estimate duration was 1h24m)",
		"Operator Health": "28 Healthy, 1 Unavailable, 4 Available but degraded",
	}

	expectedControlPlaneOperators := []string{
		"machine-config   1h4m41s   -        Working towards 4.14.1",
	}

	expectedControlPlaneNodes := []string{
		"ip-10-0-30-217.us-east-2.compute.internal   Outdated     Pending   4.14.0-rc.3   ?",
		"ip-10-0-53-40.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?",
		"ip-10-0-92-180.us-east-2.compute.internal   Outdated     Pending   4.14.0-rc.3   ?",
	}

	expectedWorkerPools := []string{
		"worker        Pending      0% (0/3)     3 Available, 0 Progressing, 0 Draining",
	}

	expectedWorkerNodes := map[string][]string{
		"worker": {
			"ip-10-0-20-162.us-east-2.compute.internal   Outdated     Pending   4.14.0-rc.3   ?",
			"ip-10-0-4-159.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?",
			"ip-10-0-99-40.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?",
		},
	}

	expectedHealth := &Health{
		Detailed: true,
		Messages: []string{
			`Message: Cluster Operator kube-apiserver is degraded (NodeController_MasterNodesReady)
  Since:       58m18s
  Level:       Error
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusteroperators.config.openshift.io: kube-apiserver
  Description: NodeControllerDegraded: The master nodes not ready: node "ip-10-0-12-74.ec2.internal" not ready since 2023-11-03 16:28:43 +0000 UTC because KubeletNotReady (container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?)`,
			`Message: Cluster Operator kube-controller-manager is degraded (NodeController_MasterNodesReady)
  Since:       58m18s
  Level:       Error
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusteroperators.config.openshift.io: kube-controller-manager
  Description: NodeControllerDegraded: The master nodes not ready: node "ip-10-0-12-74.ec2.internal" not ready since 2023-11-03 16:28:43 +0000 UTC because KubeletNotReady (container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?)`,
			`Message: Cluster Operator kube-scheduler is degraded (NodeController_MasterNodesReady)
  Since:       58m18s
  Level:       Error
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusteroperators.config.openshift.io: kube-scheduler
  Description: NodeControllerDegraded: The master nodes not ready: node "ip-10-0-12-74.ec2.internal" not ready since 2023-11-03 16:28:43 +0000 UTC because KubeletNotReady (container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?)`,
			`Message: Cluster Operator etcd is degraded (EtcdEndpoints_ErrorUpdatingEtcdEndpoints::EtcdMembers_UnhealthyMembers::NodeController_MasterNodesReady)
  Since:       58m38s
  Level:       Error
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusteroperators.config.openshift.io: etcd
  Description: EtcdEndpointsDegraded: EtcdEndpointsController can't evaluate whether quorum is safe: etcd cluster has quorum of 2 and 2 healthy members which is not fault tolerant: [{Member:ID:12895393557789359222 name:"ip-10-0-73-118.ec2.internal" peerURLs:"https://10.0.73.118:2380" clientURLs:"https://10.0.73.118:2379"  Healthy:true Took:1.725492ms Error:<nil>} {Member:ID:13608765340770574953 name:"ip-10-0-0-60.ec2.internal" peerURLs:"https://10.0.0.60:2380" clientURLs:"https://10.0.0.60:2379"  Healthy:true Took:1.542919ms Error:<nil>} {Member:ID:18044478200504924924 name:"ip-10-0-12-74.ec2.internal" peerURLs:"https://10.0.12.74:2380" clientURLs:"https://10.0.12.74:2379"  Healthy:false Took: Error:create client failure: failed to make etcd client for endpoints [https://10.0.12.74:2379]: context deadline exceeded}]
               , EtcdMembersDegraded: 2 of 3 members are available, ip-10-0-12-74.ec2.internal is unhealthy
               , NodeControllerDegraded: The master nodes not ready: node "ip-10-0-12-74.ec2.internal" not ready since 2023-11-03 16:28:43 +0000 UTC because KubeletNotReady (container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?)`,
			`Message: Cluster Operator control-plane-machine-set is unavailable (UnavailableReplicas)
  Since:       1h0m17s
  Level:       Error
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDown.md
  Resources:
    clusteroperators.config.openshift.io: control-plane-machine-set
  Description: Missing 1 available replica(s)`,
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

	output, err := newUpgradeStatusOutput(input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !output.updating {
		t.Error("Expected IsUpdating() to return true for full input case")
	}

	if output.controlPlane == nil {
		t.Fatal("Expected ControlPlane() to return non-nil object")
	}

	if diff := cmp.Diff(expectedControlPlaneSummary, output.controlPlane.Summary); diff != "" {
		t.Errorf("ControlPlane summary mismatch (-expected +actual):\n%s", diff)
	}

	if diff := cmp.Diff(expectedControlPlaneOperators, output.controlPlane.Operators); diff != "" {
		t.Errorf("ControlPlane operators mismatch (-expected +actual):\n%s", diff)
	}

	if diff := cmp.Diff(expectedControlPlaneNodes, output.controlPlane.Nodes); diff != "" {
		t.Errorf("ControlPlane nodes mismatch (-expected +actual):\n%s", diff)
	}

	if output.workers == nil {
		t.Fatal("Expected Workers() to return non-nil object")
	}

	if diff := cmp.Diff(expectedWorkerPools, output.workers.Pools); diff != "" {
		t.Errorf("Worker pools mismatch (-expected +actual):\n%s", diff)
	}

	if diff := cmp.Diff(expectedWorkerNodes, output.workers.Nodes); diff != "" {
		t.Errorf("Worker nodes mismatch (-expected +actual):\n%s", diff)
	}

	if diff := cmp.Diff(expectedHealth, output.health); diff != "" {
		t.Errorf("Health messages mismatch (-expected +actual):\n%s", diff)
	}
}

func TestUpgradeStatusOutput_NoOperatorsSection(t *testing.T) {
	input := `= Control Plane =
Assessment:      Progressing
Target Version:  4.17.0-ec.0 (from 4.16.0-0.nightly-2024-08-01-082745)
Completion:      6% (2 operators updated, 1 updating, 30 waiting)
Duration:        2m54s (Est. Time Remaining: 1h9m)
Operator Health: 32 Healthy, 1 Available but degraded

Control Plane Nodes
NAME                        ASSESSMENT   PHASE     VERSION                              EST   MESSAGE
ip-10-0-8-37.ec2.internal   Outdated     Pending   4.16.0-0.nightly-2024-08-01-082745   ?     

= Worker Upgrade =

WORKER POOL   ASSESSMENT   COMPLETION   STATUS
worker        Complete     100% (3/3)   3 Available, 0 Progressing, 0 Draining

Worker Pool Nodes: worker
NAME                                        ASSESSMENT   PHASE     VERSION       EST   MESSAGE
ip-10-0-20-162.us-east-2.compute.internal   Completed    Updated   4.17.0-ec.0   -
ip-10-0-4-159.us-east-2.compute.internal    Completed    Updated   4.17.0-ec.0   -
ip-10-0-99-40.us-east-2.compute.internal    Completed    Updated   4.17.0-ec.0   -

= Update Health =
Message: Cluster Operator kube-apiserver is degraded (NodeController_MasterNodesReady)
  Since:       58m18s
  Level:       Error
  Impact:      API Availability
  Reference:   https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md
  Resources:
    clusteroperators.config.openshift.io: kube-apiserver
  Description: NodeControllerDegraded: The master nodes not ready: node "ip-10-0-12-74.ec2.internal" not ready since 2023-11-03 16:28:43 +0000 UTC because KubeletNotReady (container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?)`

	expectedControlPlaneSummary := map[string]string{
		"Assessment":      "Progressing",
		"Target Version":  "4.17.0-ec.0 (from 4.16.0-0.nightly-2024-08-01-082745)",
		"Completion":      "6% (2 operators updated, 1 updating, 30 waiting)",
		"Duration":        "2m54s (Est. Time Remaining: 1h9m)",
		"Operator Health": "32 Healthy, 1 Available but degraded",
	}

	expectedControlPlaneNodes := []string{
		"ip-10-0-8-37.ec2.internal   Outdated     Pending   4.16.0-0.nightly-2024-08-01-082745   ?",
	}

	output, err := newUpgradeStatusOutput(input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if output.controlPlane == nil {
		t.Fatal("Expected ControlPlane() to return non-nil object")
	}

	if diff := cmp.Diff(expectedControlPlaneSummary, output.controlPlane.Summary); diff != "" {
		t.Errorf("ControlPlane summary mismatch (-expected +actual):\n%s", diff)
	}

	if output.controlPlane.Operators != nil {
		t.Errorf("Expected Operators() to return nil when section is missing, got: %v", output.controlPlane.Operators)
	}

	if diff := cmp.Diff(expectedControlPlaneNodes, output.controlPlane.Nodes); diff != "" {
		t.Errorf("ControlPlane nodes mismatch (-expected +actual):\n%s", diff)
	}

	if output.workers == nil {
		t.Fatal("Expected Workers() to return non-nil object")
	}

	expectedWorkerPools := []string{
		"worker        Complete     100% (3/3)   3 Available, 0 Progressing, 0 Draining",
	}

	if diff := cmp.Diff(expectedWorkerPools, output.workers.Pools); diff != "" {
		t.Errorf("Worker pools mismatch (-expected +actual):\n%s", diff)
	}

	expectedWorkerNodes := map[string][]string{
		"worker": {
			"ip-10-0-20-162.us-east-2.compute.internal   Completed    Updated   4.17.0-ec.0   -",
			"ip-10-0-4-159.us-east-2.compute.internal    Completed    Updated   4.17.0-ec.0   -",
			"ip-10-0-99-40.us-east-2.compute.internal    Completed    Updated   4.17.0-ec.0   -",
		},
	}

	if diff := cmp.Diff(expectedWorkerNodes, output.workers.Nodes); diff != "" {
		t.Errorf("Worker nodes mismatch (-expected +actual):\n%s", diff)
	}
}
