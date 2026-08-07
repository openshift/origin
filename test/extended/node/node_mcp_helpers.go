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

	corev1 "k8s.io/api/core/v1"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var standardNodeRoles = map[string]bool{
	"worker":        true,
	"control-plane": true,
	"master":        true,
}

func NodeHasCustomRole(node corev1.Node) (string, bool) {
	const prefix = "node-role.kubernetes.io/"
	for label := range node.Labels {
		if strings.HasPrefix(label, prefix) {
			role := strings.TrimPrefix(label, prefix)
			if !standardNodeRoles[role] {
				return role, true
			}
		}
	}
	return "", false
}

func EnsureNodeHasNoCustomRole(ctx context.Context, oc *exutil.CLI, nodeName string) error {
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}
	if role, found := NodeHasCustomRole(*node); found {
		return fmt.Errorf("node %s already has custom role %q; another test's MCP cleanup may not have completed", nodeName, role)
	}
	return nil
}

// CustomMCPConfig holds state needed to clean up a custom MCP created by CreateCustomMCPForNode.
type CustomMCPConfig struct {
	Name       string
	NodeName   string
	MCClient   *machineconfigclient.Clientset
	KubeClient *exutil.CLI
}

// CreateCustomMCPForNode labels a node and creates a custom MCP targeting it.
func CreateCustomMCPForNode(ctx context.Context, oc *exutil.CLI, mcClient *machineconfigclient.Clientset, mcpName, nodeName string) (*CustomMCPConfig, error) {
	config := &CustomMCPConfig{
		Name:       mcpName,
		NodeName:   nodeName,
		MCClient:   mcClient,
		KubeClient: oc,
	}

	if err := EnsureNodeHasNoCustomRole(ctx, oc, nodeName); err != nil {
		return nil, err
	}

	nodeLabel := fmt.Sprintf("node-role.kubernetes.io/%s", mcpName)

	framework.Logf("Labeling node %s with %s", nodeName, nodeLabel)
	patchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:""}}}`, nodeLabel))
	_, err := oc.AdminKubeClient().CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, patchData, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to label node %s: %w", nodeName, err)
	}

	framework.Logf("Creating custom MachineConfigPool %s", mcpName)
	mcp := &machineconfigv1.MachineConfigPool{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "machineconfiguration.openshift.io/v1",
			Kind:       "MachineConfigPool",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: mcpName,
			Labels: map[string]string{
				"machineconfiguration.openshift.io/pool":                      mcpName,
				"pools.operator.machineconfiguration.openshift.io/" + mcpName: "",
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
		framework.Logf("MCP creation failed, removing node label")
		unlabelPatchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, nodeLabel))
		_, _ = oc.AdminKubeClient().CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, unlabelPatchData, metav1.PatchOptions{})
		return nil, fmt.Errorf("failed to create MachineConfigPool %s: %w", mcpName, err)
	}

	framework.Logf("Waiting for custom MachineConfigPool %s to be ready", mcpName)
	err = WaitForMCP(ctx, mcClient, mcpName, 5*time.Minute)
	if err != nil {
		return config, fmt.Errorf("MachineConfigPool %s did not become ready: %w", mcpName, err)
	}

	framework.Logf("Custom MachineConfigPool %s created successfully", mcpName)
	return config, nil
}

// CleanupCustomMCP removes the node label, deletes the custom MCP, and waits for the worker MCP to stabilize.
func CleanupCustomMCP(ctx context.Context, config *CustomMCPConfig) error {
	if config == nil {
		return nil
	}

	nodeLabel := fmt.Sprintf("node-role.kubernetes.io/%s", config.Name)
	var cleanupErrors []error

	framework.Logf("Removing node label %s from node %s", nodeLabel, config.NodeName)
	patchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, nodeLabel))
	_, err := config.KubeClient.AdminKubeClient().CoreV1().Nodes().Patch(ctx, config.NodeName, types.MergePatchType, patchData, metav1.PatchOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to remove label from node %s: %w", config.NodeName, err))
	}

	if err == nil || apierrors.IsNotFound(err) {
		framework.Logf("Waiting for node %s to transition back to worker pool", config.NodeName)
		transitionErr := wait.PollUntilContextTimeout(ctx, 10*time.Second, 7*time.Minute, true, func(ctx context.Context) (bool, error) {
			node, getErr := config.KubeClient.AdminKubeClient().CoreV1().Nodes().Get(ctx, config.NodeName, metav1.GetOptions{})
			if apierrors.IsNotFound(getErr) {
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

	framework.Logf("Deleting custom MachineConfigPool %s", config.Name)
	deleteErr := config.MCClient.MachineconfigurationV1().MachineConfigPools().Delete(ctx, config.Name, metav1.DeleteOptions{})
	if deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to delete MachineConfigPool %s: %w", config.Name, deleteErr))
	}

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
