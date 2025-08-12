package admupgradestatus

import (
	"testing"
)

func TestUpgradeStatusOutput_NotUpdating(t *testing.T) {
	input := "The cluster is not updating."

	output, err := NewUpgradeStatusOutput(input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if output.IsUpdating() {
		t.Error("Expected IsUpdating() to return false for 'not updating' case")
	}

	if output.ControlPlane() != "" {
		t.Error("Expected empty ControlPlane() for 'not updating' case")
	}

	if output.Workers() != "" {
		t.Error("Expected empty Workers() for 'not updating' case")
	}

	if output.Health() != "" {
		t.Error("Expected empty Health() for 'not updating' case")
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

	expectedControlPlane := `Assessment:      Stalled
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
ip-10-0-92-180.us-east-2.compute.internal   Outdated     Pending   4.14.0-rc.3   ?`

	expectedWorkers := `WORKER POOL   ASSESSMENT   COMPLETION   STATUS
worker        Pending      0% (0/3)     3 Available, 0 Progressing, 0 Draining

Worker Pool Nodes: worker
NAME                                        ASSESSMENT   PHASE     VERSION       EST   MESSAGE
ip-10-0-20-162.us-east-2.compute.internal   Outdated     Pending   4.14.0-rc.3   ?     
ip-10-0-4-159.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?     
ip-10-0-99-40.us-east-2.compute.internal    Outdated     Pending   4.14.0-rc.3   ?`

	expectedHealth := `Message: Cluster Operator kube-apiserver is degraded (NodeController_MasterNodesReady)
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

	output, err := NewUpgradeStatusOutput(input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !output.IsUpdating() {
		t.Error("Expected IsUpdating() to return true for full input case")
	}

	controlPlane := output.ControlPlane()
	if controlPlane != expectedControlPlane {
		t.Errorf("Expected ControlPlane() to match exactly.\nGot:\n%q\nExpected:\n%q", controlPlane, expectedControlPlane)
	}

	workers := output.Workers()
	if workers != expectedWorkers {
		t.Errorf("Expected Workers() to match exactly.\nGot:\n%q\nExpected:\n%q", workers, expectedWorkers)
	}

	health := output.Health()
	if health != expectedHealth {
		t.Errorf("Expected Health() to match exactly.\nGot:\n%q\nExpected:\n%q", health, expectedHealth)
	}
}
