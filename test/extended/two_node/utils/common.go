// Package utils provides common cluster utilities: topology validation, CLI management, node filtering, and operator health checks.
package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	"github.com/openshift/origin/test/extended/util"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/test/e2e/framework"
	nodehelper "k8s.io/kubernetes/test/e2e/framework/node"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

const (
	AllNodes                  = ""                                      // No label filter for GetNodes
	LabelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane" // Control plane node label
	LabelNodeRoleWorker       = "node-role.kubernetes.io/worker"        // Worker node label
	LabelNodeRoleArbiter      = "node-role.kubernetes.io/arbiter"       // Arbiter node label
	CLIPrivilegeNonAdmin      = false                                   // Standard user CLI
	CLIPrivilegeAdmin         = true                                    // Admin CLI with cluster-admin permissions

	clusterIsHealthyTimeout = 5 * time.Minute
	pollInterval            = 5 * time.Second
)

// DecodeObject decodes YAML or JSON data into a Kubernetes runtime object using generics.
//
//	var bmh metal3v1alpha1.BareMetalHost
//	if err := DecodeObject(yamlData, &bmh); err != nil { return err }
func DecodeObject[T runtime.Object](data string, target T) error {
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(data), 4096)
	return decoder.Decode(target)
}

// SkipIfNotTopology skips the test if cluster topology doesn't match the wanted mode (e.g., DualReplicaTopologyMode).
//
//	SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)
func SkipIfNotTopology(oc *exutil.CLI, wanted v1.TopologyMode) {
	current, err := exutil.GetControlPlaneTopology(oc)
	if err != nil {
		e2eskipper.Skip(fmt.Sprintf("Could not get current topology, skipping test: error %v", err))
	}
	if *current != wanted {
		e2eskipper.Skip(fmt.Sprintf("Cluster is not in %v topology, skipping test", wanted))
	}
}

func SkipIfClusterIsNotHealthy(oc *util.CLI, ecf *helpers.EtcdClientFactoryImpl, nodes *corev1.NodeList) {
	err := ensureEtcdPodsAreRunning(oc)
	if err != nil {
		e2eskipper.Skip(fmt.Sprintf("could not ensure etcd pods are running: %v", err))
	}

	err = ensureEtcdHasTwoVotingMembers(nodes, ecf)
	if err != nil {
		e2eskipper.Skip(fmt.Sprintf("could not ensure etcd has two voting members: %v", err))
	}

	err = ensureClusterOperatorHealthy(oc)
	if err != nil {
		e2eskipper.Skip(fmt.Sprintf("could not ensure cluster-operator is healthy: %v", err))
	}
}

// IsClusterOperatorAvailable returns true if operator has Available=True condition.
//
//	if !IsClusterOperatorAvailable(etcdOperator) { return fmt.Errorf("etcd not available") }
func IsClusterOperatorAvailable(operator *v1.ClusterOperator) bool {
	for _, cond := range operator.Status.Conditions {
		if cond.Type == v1.OperatorAvailable && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

// IsClusterOperatorDegraded returns true if operator has Degraded=True condition.
//
//	if IsClusterOperatorDegraded(co) { return fmt.Errorf("%s degraded", coName) }
func IsClusterOperatorDegraded(operator *v1.ClusterOperator) bool {
	for _, cond := range operator.Status.Conditions {
		if cond.Type == v1.OperatorDegraded && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

// GetNodes returns nodes filtered by role label (LabelNodeRoleControlPlane, LabelNodeRoleWorker, etc), or all nodes if roleLabel is AllNodes.
//
//	controlPlaneNodes, err := GetNodes(oc, LabelNodeRoleControlPlane)
func GetNodes(oc *exutil.CLI, roleLabel string) (*corev1.NodeList, error) {
	return oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: roleLabel,
	})
}

// IsNodeReady checks if a node exists and is in Ready state.
// Returns true if the node exists and has Ready condition, false otherwise.
//
//	if !IsNodeReady(oc, "master-0") { /* node not ready, approve CSRs */ }
func IsNodeReady(oc *exutil.CLI, nodeName string) bool {
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		// Node doesn't exist or error retrieving it
		return false
	}

	// Check node conditions for Ready status
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

// HasNodeRebooted checks if a node has rebooted by comparing its current BootID with a previous snapshot.
// Returns true if the node's BootID has changed, indicating a reboot occurred.
//
//	nodeSnapshot, _ := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
//	// ... trigger reboot ...
//	if rebooted, err := HasNodeRebooted(oc, nodeSnapshot); rebooted { /* node rebooted */ }
func HasNodeRebooted(oc *exutil.CLI, node *corev1.Node) (bool, error) {
	if n, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{}); err != nil {
		return false, err
	} else {
		return n.Status.NodeInfo.BootID != node.Status.NodeInfo.BootID, nil
	}
}

// IsAPIResponding checks if the Kubernetes API server is responding to requests.
// Returns true if the API responds successfully, false otherwise.
//
//	if !IsAPIResponding(oc) { /* API not ready, continue waiting */ }
func IsAPIResponding(oc *exutil.CLI) bool {
	// Try a simple API call to check if the server is responding
	// Using a lightweight list operation with limit=1 to test API availability
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{Limit: 1})
	return err == nil
}

// UnmarshalJSON parses JSON string into a Go type using generics.
//
//	var node corev1.Node
//	if err := UnmarshalJSON(nodeJSON, &node); err != nil { return err }
func UnmarshalJSON[T any](jsonData string, target *T) error {
	return json.Unmarshal([]byte(jsonData), target)
}

// IsClusterHealthy checks if the cluster is in a healthy state before running disruptive tests.
// It verifies that all nodes are ready and all cluster operators are available (not degraded or progressing).
// Returns an error with details if the cluster is not healthy, nil if healthy.
//
//	if err := IsClusterHealthy(oc); err != nil {
//		e2eskipper.Skipf("Cluster is not healthy: %v", err)
//	}
func IsClusterHealthy(oc *exutil.CLI) error {
	ctx := context.Background()
	timeout := 30 * time.Second // Quick check, not a long wait

	// Check all nodes are ready first using upstream framework function
	klog.V(2).Infof("Checking if all nodes are ready...")
	if err := nodehelper.AllNodesReady(ctx, oc.AdminKubeClient(), timeout); err != nil {
		return fmt.Errorf("not all nodes are ready: %w", err)
	}
	klog.V(2).Infof("All nodes are ready")

	// Check all cluster operators using MonitorClusterOperators
	klog.V(2).Infof("Checking if all cluster operators are healthy...")
	_, err := MonitorClusterOperators(oc, timeout, 5*time.Second)
	if err != nil {
		return fmt.Errorf("cluster operators not healthy: %w", err)
	}
	klog.V(2).Infof("All cluster operators are healthy")

	return nil
}

// MonitorClusterOperators monitors cluster operators and ensures they are all available.
// Returns the cluster operators status output and an error if operators are not healthy within timeout.
//
//	output, err := MonitorClusterOperators(oc, 5*time.Minute, 15*time.Second)
func MonitorClusterOperators(oc *exutil.CLI, timeout time.Duration, pollInterval time.Duration) (string, error) {
	ctx := context.Background()
	startTime := time.Now()

	for {
		// Get cluster operators status
		clusterOperators, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.V(4).Infof("Error getting cluster operators: %v", err)
			if time.Since(startTime) >= timeout {
				return "", fmt.Errorf("timeout waiting for cluster operators: %w", err)
			}
			time.Sleep(pollInterval)
			continue
		}

		// Check each operator's conditions
		allHealthy := true
		var degradedOperators []string
		var progressingOperators []string

		for _, co := range clusterOperators.Items {
			isDegraded := false
			isProgressing := false

			// Check conditions
			for _, condition := range co.Status.Conditions {
				if condition.Type == "Degraded" && condition.Status == "True" {
					isDegraded = true
					degradedOperators = append(degradedOperators, fmt.Sprintf("%s: %s (reason: %s)", co.Name, condition.Message, condition.Reason))
				}
				if condition.Type == "Progressing" && condition.Status == "True" {
					isProgressing = true
					progressingOperators = append(progressingOperators, fmt.Sprintf("%s: %s (reason: %s)", co.Name, condition.Message, condition.Reason))
				}
			}

			if isDegraded || isProgressing {
				allHealthy = false
			}
		}

		// Log current status
		klog.V(4).Infof("Cluster operators status check: All healthy: %v, Degraded count: %d, Progressing count: %d",
			allHealthy, len(degradedOperators), len(progressingOperators))

		if len(degradedOperators) > 0 {
			klog.V(4).Infof("Degraded operators: %v", degradedOperators)
		}
		if len(progressingOperators) > 0 {
			klog.V(4).Infof("Progressing operators: %v", progressingOperators)
		}

		// If all operators are healthy, we're done
		if allHealthy {
			klog.V(2).Infof("All cluster operators are healthy (not degraded or progressing)!")
			// Get final wide output for display purposes
			wideOutput, _ := oc.AsAdmin().Run("get").Args("co", "-o", "wide").Output()
			return wideOutput, nil
		}

		// Check timeout
		if time.Since(startTime) >= timeout {
			// Get final wide output for display purposes
			wideOutput, _ := oc.AsAdmin().Run("get").Args("co", "-o", "wide").Output()
			klog.V(4).Infof("Final cluster operators status after timeout:\n%s", wideOutput)
			return wideOutput, fmt.Errorf("cluster operators did not become healthy within %v", timeout)
		}

		// Log the current operator status for debugging
		if klog.V(4).Enabled() {
			wideOutput, _ := oc.AsAdmin().Run("get").Args("co", "-o", "wide").Output()
			klog.V(4).Infof("Current cluster operators status:\n%s", wideOutput)
		}

		time.Sleep(pollInterval)
	}
}

// EnsureTNFDegradedOrSkip skips the test if the cluster is not in TNF degraded mode
// (DualReplica topology with exactly one Ready control-plane node).
func EnsureTNFDegradedOrSkip(oc *exutil.CLI) {
	SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)

	nodeList, err := GetNodes(oc, LabelNodeRoleControlPlane)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to list master nodes")

	masters := nodeList.Items

	if len(masters) != 2 {
		g.Skip(fmt.Sprintf(
			"expect exactly 2 master nodes, found %d",
			len(masters),
		))
	}

	readyCount := CountReadyNodes(masters)
	if readyCount != 1 {
		g.Skip(fmt.Sprintf(
			"cluster is not TNF degraded mode (expected exactly 1 Ready master node, got %d)",
			readyCount,
		))
	}
}

// CountReadyNodes returns the number of nodes in Ready state.
func CountReadyNodes(nodes []corev1.Node) int {
	ready := 0
	for _, n := range nodes {
		if isNodeObjReady(n) {
			ready++
		}
	}
	return ready
}

// GetReadyMasterNode returns the first Ready control-plane node.
func GetReadyMasterNode(
	ctx context.Context,
	oc *exutil.CLI,
) (*corev1.Node, error) {
	nodeList, err := GetNodes(oc, LabelNodeRoleControlPlane)
	if err != nil {
		return nil, err
	}
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if isNodeObjReady(nodeList.Items[i]) {
			return node, nil
		}
	}

	return nil, fmt.Errorf("no Ready control-plane node found")
}

// check ready condition on an existing Node object.
func isNodeObjReady(node corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func GetMembers(etcdClientFactory helpers.EtcdClientCreator) ([]*etcdserverpb.Member, error) {
	etcdClient, closeFn, err := etcdClientFactory.NewEtcdClient()
	if err != nil {
		return []*etcdserverpb.Member{}, errors.Wrap(err, "could not get a etcd client")
	}
	defer closeFn()

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()
	m, err := etcdClient.MemberList(ctx)
	if err != nil {
		return []*etcdserverpb.Member{}, errors.Wrap(err, "could not get the member list")
	}
	return m.Members, nil
}

func GetMemberState(node *corev1.Node, members []*etcdserverpb.Member) (started, learner bool, err error) {
	// Etcd members that have been added to the member list but haven't
	// joined yet will have an empty Name field. We can match them via Peer URL.
	hostPort := net.JoinHostPort(node.Status.Addresses[0].Address, "2380")
	peerURL := fmt.Sprintf("https://%s", hostPort)
	var found bool
	for _, m := range members {
		if m.Name == node.Name {
			found = true
			started = true
			learner = m.IsLearner
			break
		}
		if slices.Contains(m.PeerURLs, peerURL) {
			found = true
			learner = m.IsLearner
			break
		}
	}
	if !found {
		return false, false, fmt.Errorf("could not find node %v via peer URL %s", node.Name, peerURL)
	}
	return started, learner, nil
}

// ensureClusterOperatorHealthy checks if the cluster-etcd-operator is healthy before running etcd tests
func ensureClusterOperatorHealthy(oc *util.CLI) error {
	framework.Logf("Ensure cluster operator is healthy (timeout: %v)", clusterIsHealthyTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), clusterIsHealthyTimeout)
	defer cancel()

	for {
		co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, "etcd", metav1.GetOptions{})
		if err != nil {
			err = fmt.Errorf("failed to retrieve ClusterOperator: %v", err)
		} else {
			// Check if etcd operator is Available
			available := findClusterOperatorCondition(co.Status.Conditions, v1.OperatorAvailable)
			if available == nil {
				err = fmt.Errorf("ClusterOperator Available condition not found")
			} else if available.Status != v1.ConditionTrue {
				err = fmt.Errorf("ClusterOperator is not Available: %s", available.Message)
			} else {
				// Check if etcd operator is not Degraded
				degraded := findClusterOperatorCondition(co.Status.Conditions, v1.OperatorDegraded)
				if degraded != nil && degraded.Status == v1.ConditionTrue {
					err = fmt.Errorf("ClusterOperator is Degraded: %s", degraded.Message)
				} else {
					framework.Logf("SUCCESS: Cluster operator is healthy")
					return nil
				}
			}
		}

		select {
		case <-ctx.Done():
			return err
		default:
		}
		time.Sleep(pollInterval)
	}
}

func ensureEtcdPodsAreRunning(oc *util.CLI) error {
	framework.Logf("Ensure Etcd pods are running (timeout: %v)", clusterIsHealthyTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), clusterIsHealthyTimeout)
	defer cancel()
	for {
		etcdPods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=etcd",
		})
		if err != nil {
			err = fmt.Errorf("failed to retrieve etcd pods: %v", err)
		} else {
			runningPods := 0
			for _, pod := range etcdPods.Items {
				if pod.Status.Phase == corev1.PodRunning {
					runningPods++
				}
			}
			if runningPods < 2 {
				return fmt.Errorf("expected at least 2 etcd pods running, found %d", runningPods)
			}

			framework.Logf("SUCCESS: found the 2 expected Etcd pods")
			return nil
		}

		select {
		case <-ctx.Done():
			return err
		default:
		}
		time.Sleep(pollInterval)
	}
}

// findClusterOperatorCondition finds a condition in ClusterOperator status
func findClusterOperatorCondition(conditions []v1.ClusterOperatorStatusCondition, conditionType v1.ClusterStatusConditionType) *v1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func ensureEtcdHasTwoVotingMembers(nodes *corev1.NodeList, ecf *helpers.EtcdClientFactoryImpl) error {
	framework.Logf("Ensure Etcd member list has two voting members (timeout: %v)", clusterIsHealthyTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), clusterIsHealthyTimeout)
	defer cancel()

	for {
		// Check all conditions sequentially
		members, err := GetMembers(ecf)
		if err == nil && len(members) != 2 {
			err = fmt.Errorf("expected 2 members, found %d", len(members))
		}

		if err == nil {
			for _, node := range nodes.Items {
				isStarted, isLearner, checkErr := GetMemberState(&node, members)
				if checkErr != nil {
					err = checkErr
				} else if !isStarted || isLearner {
					err = fmt.Errorf("member %s is not a voting member (started=%v, learner=%v)",
						node.Name, isStarted, isLearner)
					break
				}
			}

		}

		// All checks passed - success!
		if err == nil {
			framework.Logf("SUCCESS: got membership with two voting members: %+v", members)
			return nil
		}

		// Checks failed - evaluate timeout
		select {
		case <-ctx.Done():
			return fmt.Errorf("etcd membership does not have two voters: %v, membership: %+v", err, members)
		default:
		}
		time.Sleep(pollInterval)
	}
}
