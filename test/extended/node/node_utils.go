package node

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

// KubeletConfiguration represents the kubelet configuration structure
type KubeletConfiguration struct {
	FailSwapOn  *bool                    `json:"failSwapOn,omitempty"`
	MemorySwap  *MemorySwapConfiguration `json:"memorySwap,omitempty"`
	SwapDesired *string                  `json:"swapDesired,omitempty"`
}

// MemorySwapConfiguration represents memory swap configuration
type MemorySwapConfiguration struct {
	SwapBehavior string `json:"swapBehavior,omitempty"`
}

// getWorkerNodes returns all worker nodes in the cluster
func getWorkerNodes(ctx context.Context, oc *exutil.CLI) ([]v1.Node, error) {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var workerNodes []v1.Node
	for _, node := range nodes.Items {
		if _, ok := node.Labels["node-role.kubernetes.io/worker"]; ok {
			workerNodes = append(workerNodes, node)
		}
	}
	return workerNodes, nil
}

// getControlPlaneNodes returns all control plane nodes in the cluster
func getControlPlaneNodes(ctx context.Context, oc *exutil.CLI) ([]v1.Node, error) {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var controlPlaneNodes []v1.Node
	for _, node := range nodes.Items {
		if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
			controlPlaneNodes = append(controlPlaneNodes, node)
		} else if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
			controlPlaneNodes = append(controlPlaneNodes, node)
		}
	}
	return controlPlaneNodes, nil
}

// getKubeletConfigFromNode retrieves the kubelet configuration from a specific node
func getKubeletConfigFromNode(ctx context.Context, oc *exutil.CLI, nodeName string) (*KubeletConfiguration, error) {
	// Use the node proxy API to get configz
	configzPath := fmt.Sprintf("/api/v1/nodes/%s/proxy/configz", nodeName)

	data, err := oc.AdminKubeClient().CoreV1().RESTClient().Get().AbsPath(configzPath).DoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get configz from node %s: %w", nodeName, err)
	}

	// Parse the JSON response
	var configzResponse struct {
		KubeletConfig *KubeletConfiguration `json:"kubeletconfig"`
	}

	if err := json.Unmarshal(data, &configzResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configz response: %w", err)
	}

	if configzResponse.KubeletConfig == nil {
		return nil, fmt.Errorf("kubeletconfig is nil in response")
	}

	return configzResponse.KubeletConfig, nil
}
