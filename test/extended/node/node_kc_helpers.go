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
func CreateKubeletConfig(ctx context.Context, mcClient *machineconfigclient.Clientset, kubeletConfig *machineconfigv1.KubeletConfig) (*machineconfigv1.KubeletConfig, error) {
	framework.Logf("Creating KubeletConfig %s", kubeletConfig.Name)
	created, err := mcClient.MachineconfigurationV1().KubeletConfigs().Create(ctx, kubeletConfig, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// CreateOrUpdateKubeletConfig creates a KubeletConfig or updates it if it already exists.
func CreateOrUpdateKubeletConfig(ctx context.Context, mcClient *machineconfigclient.Clientset, kubeletConfig *machineconfigv1.KubeletConfig) (*machineconfigv1.KubeletConfig, error) {
	existing, err := mcClient.MachineconfigurationV1().KubeletConfigs().Get(ctx, kubeletConfig.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		framework.Logf("Creating KubeletConfig %s", kubeletConfig.Name)
		return mcClient.MachineconfigurationV1().KubeletConfigs().Create(ctx, kubeletConfig, metav1.CreateOptions{})
	}
	if err != nil {
		return nil, err
	}

	framework.Logf("Updating existing KubeletConfig %s", kubeletConfig.Name)
	existing.Spec = kubeletConfig.Spec
	return mcClient.MachineconfigurationV1().KubeletConfigs().Update(ctx, existing, metav1.UpdateOptions{})
}

// CleanupKubeletConfig deletes a KubeletConfig. If mcpName is non-empty, waits for that MCP to stabilize.
func CleanupKubeletConfig(ctx context.Context, mcClient *machineconfigclient.Clientset, kcName, mcpName string) error {
	framework.Logf("Cleaning up KubeletConfig %s", kcName)

	deleteErr := mcClient.MachineconfigurationV1().KubeletConfigs().Delete(ctx, kcName, metav1.DeleteOptions{})
	if deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
		return deleteErr
	}

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

// ApplyKubeletConfigAndWaitForMCP creates a KubeletConfig and waits for the MCP rollout to complete.
func ApplyKubeletConfigAndWaitForMCP(ctx context.Context, mcClient *machineconfigclient.Clientset, kubeletConfig *machineconfigv1.KubeletConfig, mcpName string, rolloutTimeout time.Duration) error {
	_, err := CreateKubeletConfig(ctx, mcClient, kubeletConfig)
	if err != nil {
		return err
	}

	framework.Logf("Waiting for MCP %s to start updating", mcpName)
	err = WaitForMCPUpdating(ctx, mcClient, mcpName, 5*time.Minute)
	if err != nil {
		return err
	}

	framework.Logf("Waiting for MCP %s to complete rollout", mcpName)
	return WaitForMCP(ctx, mcClient, mcpName, rolloutTimeout)
}

// WaitForMCPUpdating waits for an MCP to enter the "Updating" state.
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
