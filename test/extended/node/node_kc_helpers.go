package node

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
)

// CreateKubeletConfig creates a KubeletConfig resource.
// Returns the created KubeletConfig for reference.
//
// Example usage:
//
//	kc, err := CreateKubeletConfig(ctx, mcClient, kubeletConfig)
//	o.Expect(err).NotTo(o.HaveOccurred())
//	defer CleanupKubeletConfig(context.Background(), mcClient, kc.Name, "")
func CreateKubeletConfig(ctx context.Context, mcClient *machineconfigclient.Clientset, kubeletConfig *machineconfigv1.KubeletConfig) (*machineconfigv1.KubeletConfig, error) {
	framework.Logf("Creating KubeletConfig %s", kubeletConfig.Name)
	created, err := mcClient.MachineconfigurationV1().KubeletConfigs().Create(ctx, kubeletConfig, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// CleanupKubeletConfig deletes a KubeletConfig and optionally waits for the associated MCP to stabilize.
// This function is idempotent and safe to call multiple times.
//
// Parameters:
//   - ctx: Context for the operation
//   - mcClient: Machine config client
//   - kcName: Name of the KubeletConfig to delete
//   - mcpName: Optional name of the MachineConfigPool to wait for after deletion.
//     If empty, no MCP wait is performed.
//
// Example usage:
//
//	// Simple cleanup without waiting for MCP
//	err := CleanupKubeletConfig(ctx, mcClient, "my-kc", "")
//
//	// Cleanup and wait for MCP to stabilize
//	err := CleanupKubeletConfig(ctx, mcClient, "my-kc", "worker")
func CleanupKubeletConfig(ctx context.Context, mcClient *machineconfigclient.Clientset, kcName, mcpName string) error {
	framework.Logf("Cleaning up KubeletConfig %s", kcName)

	// Delete the KubeletConfig
	deleteErr := mcClient.MachineconfigurationV1().KubeletConfigs().Delete(ctx, kcName, metav1.DeleteOptions{})
	if deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
		return deleteErr
	}

	// If MCP name is provided, wait for it to stabilize
	if mcpName != "" && (deleteErr == nil || apierrors.IsNotFound(deleteErr)) {
		framework.Logf("Waiting for MCP %s to become ready after KubeletConfig deletion", mcpName)
		waitErr := WaitForMCP(ctx, mcClient, mcpName, 15*time.Minute)
		if waitErr != nil && !apierrors.IsNotFound(waitErr) {
			return waitErr
		}
	}

	framework.Logf("KubeletConfig %s cleaned up successfully", kcName)
	return nil
}

// ApplyKubeletConfigAndWaitForMCP creates a KubeletConfig and waits for the specified MCP to complete rollout.
// This is a common pattern in tests that apply KubeletConfig changes.
//
// Parameters:
//   - ctx: Context for the operation
//   - mcClient: Machine config client
//   - kubeletConfig: The KubeletConfig to create
//   - mcpName: Name of the MachineConfigPool to wait for
//   - rolloutTimeout: How long to wait for the full rollout (default: 15 minutes)
//
// Example usage:
//
//	err := ApplyKubeletConfigAndWaitForMCP(ctx, mcClient, kubeletConfig, "worker", 15*time.Minute)
//	o.Expect(err).NotTo(o.HaveOccurred())
func ApplyKubeletConfigAndWaitForMCP(ctx context.Context, mcClient *machineconfigclient.Clientset, kubeletConfig *machineconfigv1.KubeletConfig, mcpName string, rolloutTimeout time.Duration) error {
	// Create the KubeletConfig
	_, err := CreateKubeletConfig(ctx, mcClient, kubeletConfig)
	if err != nil {
		return err
	}

	// Wait for MCP to start updating
	framework.Logf("Waiting for MCP %s to start updating", mcpName)
	err = WaitForMCPUpdating(ctx, mcClient, mcpName, 5*time.Minute)
	if err != nil {
		return err
	}

	// Wait for MCP to complete rollout
	framework.Logf("Waiting for MCP %s to complete rollout", mcpName)
	return WaitForMCP(ctx, mcClient, mcpName, rolloutTimeout)
}

// WaitForMCPUpdating waits for a MachineConfigPool to enter the "Updating" state.
// This is useful to confirm that a configuration change has been picked up by MCO.
//
// Parameters:
//   - ctx: Context for the operation
//   - mcClient: Machine config client
//   - mcpName: Name of the MachineConfigPool to watch
//   - timeout: How long to wait for the update to start
//
// Returns error if timeout expires or MCP not found.
func WaitForMCPUpdating(ctx context.Context, mcClient *machineconfigclient.Clientset, mcpName string, timeout time.Duration) error {
	startTime := time.Now()
	for {
		mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, mcpName, metav1.GetOptions{})
		if err != nil {
			if time.Since(startTime) > timeout {
				return err
			}
			framework.Logf("Error getting MCP %s: %v, retrying...", mcpName, err)
			time.Sleep(10 * time.Second)
			continue
		}

		for _, condition := range mcp.Status.Conditions {
			if condition.Type == "Updating" && condition.Status == "True" {
				framework.Logf("MCP %s has started updating", mcpName)
				return nil
			}
		}

		if time.Since(startTime) > timeout {
			return fmt.Errorf("timeout waiting for MCP %s to start updating", mcpName)
		}

		time.Sleep(10 * time.Second)
	}
}
