// Package utils provides common cluster utilities: topology validation, CLI management, node filtering, and operator health checks.
package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	v1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

const (
	AllNodes                  = ""                                      // No label filter for GetNodes
	LabelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane" // Control plane node label
	LabelNodeRoleWorker       = "node-role.kubernetes.io/worker"        // Worker node label
	LabelNodeRoleArbiter      = "node-role.kubernetes.io/arbiter"       // Arbiter node label
	CLIPrivilegeNonAdmin      = false                                   // Standard user CLI
	CLIPrivilegeAdmin         = true                                    // Admin CLI with cluster-admin permissions
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
