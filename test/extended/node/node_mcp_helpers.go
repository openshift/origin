package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

// CustomMCPConfig holds the configuration for a custom MachineConfigPool setup.
// This is returned by CreateCustomMCPForNode and should be passed to CleanupCustomMCP.
type CustomMCPConfig struct {
	// Name is the name of the custom MachineConfigPool
	Name string
	// NodeName is the name of the node labeled and added to the MCP
	NodeName string
	// MCClient is the machine config client for MCP operations
	MCClient *machineconfigclient.Clientset
	// KubeClient is the Kubernetes client for node operations
	KubeClient *exutil.CLI
}

// CreateCustomMCPForNode creates a custom MachineConfigPool and labels a node to join it.
// It returns a CustomMCPConfig that should be passed to CleanupCustomMCP for cleanup.
// This is useful for tests that need to apply custom KubeletConfigs to specific nodes
// without affecting the entire worker pool.
//
// The function performs the following steps:
// 1. Labels the specified node with "node-role.kubernetes.io/<mcpName>"
// 2. Creates a custom MachineConfigPool that targets nodes with that label
// 3. Waits for the MCP to become ready (up to 5 minutes)
//
// Example usage:
//
//	mcClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
//	o.Expect(err).NotTo(o.HaveOccurred())
//
//	mcpConfig, err := CreateCustomMCPForNode(ctx, oc, mcClient, "my-test-pool", nodeName)
//	o.Expect(err).NotTo(o.HaveOccurred())
//	defer CleanupCustomMCP(context.Background(), mcpConfig)
//
//	// Now you can create KubeletConfigs targeting this MCP
//	// The node will apply configs without affecting other workers
func CreateCustomMCPForNode(ctx context.Context, oc *exutil.CLI, mcClient *machineconfigclient.Clientset, mcpName, nodeName string) (*CustomMCPConfig, error) {
	config := &CustomMCPConfig{
		Name:       mcpName,
		NodeName:   nodeName,
		MCClient:   mcClient,
		KubeClient: oc,
	}

	nodeLabel := fmt.Sprintf("node-role.kubernetes.io/%s", mcpName)

	// Step 1: Label the node
	framework.Logf("Labeling node %s with %s", nodeName, nodeLabel)
	patchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:""}}}`, nodeLabel))
	_, err := oc.AdminKubeClient().CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, patchData, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to label node %s: %w", nodeName, err)
	}

	// Step 2: Create custom MachineConfigPool
	framework.Logf("Creating custom MachineConfigPool %s", mcpName)
	mcp := &machineconfigv1.MachineConfigPool{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "machineconfiguration.openshift.io/v1",
			Kind:       "MachineConfigPool",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: mcpName,
			Labels: map[string]string{
				"machineconfiguration.openshift.io/pool": mcpName,
			},
		},
		Spec: machineconfigv1.MachineConfigPoolSpec{
			MachineConfigSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "machineconfiguration.openshift.io/role",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"worker", mcpName},
					},
				},
			},
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					nodeLabel: "",
				},
			},
		},
	}

	_, err = mcClient.MachineconfigurationV1().MachineConfigPools().Create(ctx, mcp, metav1.CreateOptions{})
	if err != nil {
		// Cleanup the node label if MCP creation fails
		framework.Logf("MCP creation failed, removing node label")
		unlabelPatchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, nodeLabel))
		_, _ = oc.AdminKubeClient().CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, unlabelPatchData, metav1.PatchOptions{})
		return nil, fmt.Errorf("failed to create MachineConfigPool %s: %w", mcpName, err)
	}

	// Step 3: Wait for MCP to become ready
	framework.Logf("Waiting for custom MachineConfigPool %s to be ready", mcpName)
	err = WaitForMCP(ctx, mcClient, mcpName, 5*time.Minute)
	if err != nil {
		return config, fmt.Errorf("MachineConfigPool %s did not become ready: %w", mcpName, err)
	}

	framework.Logf("Custom MachineConfigPool %s created successfully", mcpName)
	return config, nil
}

// CleanupCustomMCP removes the custom MachineConfigPool and unlabels the node,
// returning it to the worker pool. It should be called in a defer or cleanup
// handler after CreateCustomMCPForNode.
//
// The function performs the following steps:
// 1. Removes the custom node label from the node
// 2. Waits for the node to transition back to the worker pool (up to 7 minutes)
// 3. Deletes the custom MachineConfigPool
// 4. Waits for the worker MCP to stabilize (up to 10 minutes)
//
// This function is idempotent and safe to call multiple times. It handles
// NotFound errors gracefully, making it safe to use even if resources were
// already cleaned up.
//
// Example usage:
//
//	mcpConfig, err := CreateCustomMCPForNode(ctx, oc, mcClient, "my-test-pool", nodeName)
//	o.Expect(err).NotTo(o.HaveOccurred())
//	defer func() {
//		err := CleanupCustomMCP(context.Background(), mcpConfig)
//		if err != nil {
//			framework.Logf("Warning: cleanup had errors: %v", err)
//		}
//	}()
func CleanupCustomMCP(ctx context.Context, config *CustomMCPConfig) error {
	if config == nil {
		return nil
	}

	nodeLabel := fmt.Sprintf("node-role.kubernetes.io/%s", config.Name)
	var cleanupErrors []error

	// Step 1: Remove node label
	framework.Logf("Removing node label %s from node %s", nodeLabel, config.NodeName)
	patchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, nodeLabel))
	_, err := config.KubeClient.AdminKubeClient().CoreV1().Nodes().Patch(ctx, config.NodeName, types.MergePatchType, patchData, metav1.PatchOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to remove label from node %s: %w", config.NodeName, err))
	}

	// Step 2: Wait for node to transition back to worker pool
	if err == nil || apierrors.IsNotFound(err) {
		framework.Logf("Waiting for node %s to transition back to worker pool", config.NodeName)
		transitionErr := wait.PollUntilContextTimeout(ctx, 10*time.Second, 7*time.Minute, true, func(ctx context.Context) (bool, error) {
			node, getErr := config.KubeClient.AdminKubeClient().CoreV1().Nodes().Get(ctx, config.NodeName, metav1.GetOptions{})
			if apierrors.IsNotFound(getErr) {
				// Node was deleted, consider it transitioned
				return true, nil
			}
			if getErr != nil {
				return false, nil
			}
			currentConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
			desiredConfig := node.Annotations["machineconfiguration.openshift.io/desiredConfig"]
			isWorkerConfig := currentConfig != "" && !strings.Contains(currentConfig, config.Name) && currentConfig == desiredConfig
			return isWorkerConfig, nil
		})
		if transitionErr != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("node %s did not transition back to worker pool: %w", config.NodeName, transitionErr))
		}
	}

	// Step 3: Delete the custom MachineConfigPool
	framework.Logf("Deleting custom MachineConfigPool %s", config.Name)
	deleteErr := config.MCClient.MachineconfigurationV1().MachineConfigPools().Delete(ctx, config.Name, metav1.DeleteOptions{})
	if deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to delete MachineConfigPool %s: %w", config.Name, deleteErr))
	}

	// Step 4: Wait for worker MCP to stabilize
	if deleteErr == nil || apierrors.IsNotFound(deleteErr) {
		framework.Logf("Waiting for worker MCP to stabilize after custom MCP deletion")
		waitErr := WaitForMCP(ctx, config.MCClient, "worker", 10*time.Minute)
		if waitErr != nil && !apierrors.IsNotFound(waitErr) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("worker MCP did not stabilize: %w", waitErr))
		}
	}

	if len(cleanupErrors) > 0 {
		return fmt.Errorf("cleanup completed with errors: %v", cleanupErrors)
	}

	framework.Logf("Custom MachineConfigPool %s cleaned up successfully", config.Name)
	return nil
}
