package node

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/kubernetes/test/e2e/framework"

	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

// getNodesByLabel returns nodes matching the specified label selector
func getNodesByLabel(ctx context.Context, oc *exutil.CLI, labelSelector string) ([]corev1.Node, error) {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}
	return nodes.Items, nil
}

// getControlPlaneNodes returns all control plane nodes in the cluster
func getControlPlaneNodes(ctx context.Context, oc *exutil.CLI) ([]corev1.Node, error) {
	// Try master label first (OpenShift uses this)
	nodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/master")
	if err != nil {
		return nil, err
	}
	if len(nodes) > 0 {
		return nodes, nil
	}

	// Fallback to control-plane label (upstream Kubernetes uses this)
	return getNodesByLabel(ctx, oc, "node-role.kubernetes.io/control-plane")
}

// getKubeletConfigFromNode retrieves the kubelet configuration from a specific node
func getKubeletConfigFromNode(ctx context.Context, oc *exutil.CLI, nodeName string) (*kubeletconfigv1beta1.KubeletConfiguration, error) {
	// Use the node proxy API to get configz
	configzPath := fmt.Sprintf("/api/v1/nodes/%s/proxy/configz", nodeName)

	data, err := oc.AdminKubeClient().CoreV1().RESTClient().Get().AbsPath(configzPath).DoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get configz from node %s: %w", nodeName, err)
	}

	// Parse the JSON response
	var configzResponse struct {
		KubeletConfig *kubeletconfigv1beta1.KubeletConfiguration `json:"kubeletconfig"`
	}

	if err := json.Unmarshal(data, &configzResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configz response: %w", err)
	}

	if configzResponse.KubeletConfig == nil {
		return nil, fmt.Errorf("kubeletconfig is nil in response")
	}

	return configzResponse.KubeletConfig, nil
}

// ExecOnNodeWithChroot runs a command on a node using oc debug with chroot /host.
func ExecOnNodeWithChroot(oc *exutil.CLI, nodeName string, cmd ...string) (string, error) {
	// The argument "-ndefault" is required as the kubeconfig has a different namespace.
	// WithoutNamespace() is not sufficient as it just makes sure that the namespace argument is not passed, it won't
	// change the namespace in the kubeconfig
	args := append([]string{"node/" + nodeName, "-ndefault", "--", "chroot", "/host"}, cmd...)
	return oc.AsAdmin().Run("debug").Args(args...).Output()
}

// waitForMCPToBeReady waits for a MachineConfigPool to be ready
func waitForMCPToBeReady(ctx context.Context, mcClient *machineconfigclient.Clientset, poolName string, timeout time.Duration) error {
	return wait.PollImmediate(10*time.Second, timeout, func() (bool, error) {
		mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// Check if all conditions are met for a ready state
		updating := false
		degraded := false
		ready := false

		for _, condition := range mcp.Status.Conditions {
			switch condition.Type {
			case "Updating":
				if condition.Status == corev1.ConditionTrue {
					updating = true
				}
			case "Degraded":
				if condition.Status == corev1.ConditionTrue {
					degraded = true
				}
			case "Updated":
				if condition.Status == corev1.ConditionTrue {
					ready = true
				}
			}
		}

		if degraded {
			return false, fmt.Errorf("MachineConfigPool %s is degraded", poolName)
		}

		// Ready when not updating and updated condition is true
		isReady := !updating && ready && mcp.Status.ReadyMachineCount == mcp.Status.MachineCount

		if isReady {
			framework.Logf("MachineConfigPool %s is ready: %d/%d machines ready",
				poolName, mcp.Status.ReadyMachineCount, mcp.Status.MachineCount)
		} else {
			framework.Logf("MachineConfigPool %s not ready yet: updating=%v, ready=%v, machines=%d/%d",
				poolName, updating, ready, mcp.Status.ReadyMachineCount, mcp.Status.MachineCount)
		}

		return isReady, nil
	})
}
