package util

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

// GetReadySchedulableWorkerNodes returns ready schedulable worker nodes.
// This function filters out nodes with NoSchedule/NoExecute taints and non-worker nodes,
// making it suitable for tests that need to select worker nodes for workload placement.
func GetReadySchedulableWorkerNodes(ctx context.Context, client kubernetes.Interface) ([]v1.Node, error) {
	// Get ready schedulable nodes (excludes nodes with NoSchedule/NoExecute taints)
	nodes, err := e2enode.GetReadySchedulableNodes(ctx, client)
	if err != nil {
		return nil, err
	}

	// Filter for worker nodes only (exclude nodes that also have master/control-plane role)
	// This ensures we only return pure worker nodes that can be moved to custom MCPs.
	// In SNO and compact clusters, nodes can have both worker and master labels but belong
	// to the master MCP and cannot be added to custom MCPs.
	var workerNodes []v1.Node
	for _, node := range nodes.Items {
		_, hasWorkerLabel := node.Labels["node-role.kubernetes.io/worker"]
		_, hasControlPlane := node.Labels["node-role.kubernetes.io/control-plane"]
		_, hasMaster := node.Labels["node-role.kubernetes.io/master"]

		if hasWorkerLabel && !hasControlPlane && !hasMaster {
			workerNodes = append(workerNodes, node)
		}
	}

	return workerNodes, nil
}
