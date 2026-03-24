package node

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"

	o "github.com/onsi/gomega"

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

// getPureWorkerNodes returns worker nodes that are not also control plane nodes.
// On SNO clusters, the single node has both worker and control-plane roles,
// so it should be validated as a control plane node (failSwapOn=true), not as a worker.
func getPureWorkerNodes(nodes []corev1.Node) []corev1.Node {
	var pureWorkers []corev1.Node
	for _, node := range nodes {
		_, hasControlPlane := node.Labels["node-role.kubernetes.io/control-plane"]
		_, hasMaster := node.Labels["node-role.kubernetes.io/master"]
		if hasControlPlane || hasMaster {
			framework.Logf("Skipping worker validation for node %s (also has control-plane role, e.g. SNO)", node.Name)
			continue
		}
		pureWorkers = append(pureWorkers, node)
	}
	return pureWorkers
}

const (
	// debugNamespace is the namespace for debug pods
	debugNamespace = "openshift-machine-config-operator"
	// cnvNamespace is the namespace for CNV operator
	cnvNamespace = "openshift-cnv"
	// cnvOperatorGroup is the name of the CNV operator group
	cnvOperatorGroup = "kubevirt-hyperconverged-group"
	// cnvSubscription is the name of the CNV subscription
	cnvSubscription = "hco-operatorhub"
	// cnvHyperConverged is the name of the HyperConverged CR
	cnvHyperConverged = "kubevirt-hyperconverged"
	// cnvNodeLabel is the label for CNV-schedulable nodes
	cnvNodeLabel = "kubevirt.io/schedulable"
)

// GVRs for CNV resources
var (
	subscriptionGVR = schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "subscriptions",
	}
	operatorGroupGVR = schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1",
		Resource: "operatorgroups",
	}
	hyperConvergedGVR = schema.GroupVersionResource{
		Group:    "hco.kubevirt.io",
		Version:  "v1beta1",
		Resource: "hyperconvergeds",
	}
	csvGVR = schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "clusterserviceversions",
	}
	mcpGVR = schema.GroupVersionResource{
		Group:    "machineconfiguration.openshift.io",
		Version:  "v1",
		Resource: "machineconfigpools",
	}
)

// getCNVWorkerNodeName returns the name of a worker node with CNV label (kubevirt.io/schedulable=true)
func getCNVWorkerNodeName(ctx context.Context, oc *exutil.CLI) string {
	// First try to get nodes with CNV schedulable label
	nodes, err := getNodesByLabel(ctx, oc, "kubevirt.io/schedulable=true")
	if err == nil && len(nodes) > 0 {
		// Randomly select a node from the available CNV nodes
		return nodes[rand.Intn(len(nodes))].Name
	}

	// Fallback to any worker node
	nodes, err = getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
	if err != nil || len(nodes) == 0 {
		return ""
	}
	// Randomly select a node from available worker nodes
	return nodes[rand.Intn(len(nodes))].Name
}

// ExecOnNodeWithChroot runs a command on a node using oc debug with chroot /host
func ExecOnNodeWithChroot(oc *exutil.CLI, nodeName string, cmd ...string) (string, error) {
	args := append([]string{"node/" + nodeName, "-n" + debugNamespace, "--", "chroot", "/host"}, cmd...)
	stdOut, _, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args(args...).Outputs()
	return stdOut, err
}

// ExecOnNodeWithNsenter runs a command on a node using nsenter to access host namespaces
// This is needed for swap operations (swapon/swapoff) that require direct namespace access
func ExecOnNodeWithNsenter(oc *exutil.CLI, nodeName string, cmd ...string) (string, error) {
	nsenterCmd := append([]string{"nsenter", "-a", "-t", "1"}, cmd...)
	args := append([]string{"node/" + nodeName, "-n" + debugNamespace, "--"}, nsenterCmd...)
	stdOut, _, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args(args...).Outputs()
	return stdOut, err
}

// createDropInFile creates a drop-in configuration file on the specified node
func createDropInFile(oc *exutil.CLI, nodeName, filePath, content string) error {
	// Escape content for shell
	escapedContent := strings.ReplaceAll(content, "'", "'\\''")
	cmd := fmt.Sprintf("echo '%s' > %s && chmod 644 %s", escapedContent, filePath, filePath)
	_, err := ExecOnNodeWithChroot(oc, nodeName, "sh", "-c", cmd)
	return err
}

// removeDropInFile removes a drop-in configuration file from the specified node
func removeDropInFile(oc *exutil.CLI, nodeName, filePath string) error {
	_, err := ExecOnNodeWithChroot(oc, nodeName, "rm", "-f", filePath)
	return err
}

// loadConfigFromFile reads kubelet configuration from a YAML file
func loadConfigFromFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		framework.Failf("Failed to read config file %s: %v", path, err)
	}
	return string(data)
}

// restartKubeletOnNode restarts the kubelet service on the specified node
func restartKubeletOnNode(oc *exutil.CLI, nodeName string) error {
	_, err := ExecOnNodeWithChroot(oc, nodeName, "systemctl", "restart", "kubelet")
	return err
}

// waitForNodeToBeReady waits for a node to become Ready
func waitForNodeToBeReady(ctx context.Context, oc *exutil.CLI, nodeName string) {
	o.Eventually(func() bool {
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return isNodeInReadyState(node)
	}, 5*time.Minute, 10*time.Second).Should(o.BeTrue(), "Node %s should become Ready", nodeName)
}

// isNodeInReadyState checks if a node is in Ready condition
func isNodeInReadyState(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// cleanupDropInAndRestartKubelet removes the drop-in file and restarts kubelet
func cleanupDropInAndRestartKubelet(ctx context.Context, oc *exutil.CLI, nodeName, filePath string) {
	framework.Logf("Removing drop-in file: %s", filePath)
	removeDropInFile(oc, nodeName, filePath)
	framework.Logf("Restarting kubelet on node: %s", nodeName)
	restartKubeletOnNode(oc, nodeName)
	framework.Logf("Waiting for node to be ready...")
	waitForNodeToBeReady(ctx, oc, nodeName)
}

// ============================================================================
// CNV Operator Installation/Uninstallation Functions
// ============================================================================

// isCNVInstalled checks if CNV operator is installed
func isCNVInstalled(ctx context.Context, oc *exutil.CLI) bool {
	// Check if CNV namespace exists
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, cnvNamespace, metav1.GetOptions{})
	if err != nil {
		return false
	}

	// Check if HyperConverged CR exists
	dynamicClient := oc.AdminDynamicClient()
	_, err = dynamicClient.Resource(hyperConvergedGVR).Namespace(cnvNamespace).Get(ctx, cnvHyperConverged, metav1.GetOptions{})
	return err == nil
}

// installCNVOperator installs the CNV operator and creates HyperConverged CR
func installCNVOperator(ctx context.Context, oc *exutil.CLI) error {
	framework.Logf("Installing CNV operator...")

	dynamicClient := oc.AdminDynamicClient()

	// Step 1: Create CNV namespace with Pod Security labels
	// CNV requires privileged access for networking DaemonSets (bridge plugins, etc.)
	framework.Logf("Creating namespace %s with Pod Security labels", cnvNamespace)

	podSecurityLabels := map[string]string{
		"pod-security.kubernetes.io/enforce":             "privileged",
		"pod-security.kubernetes.io/audit":               "privileged",
		"pod-security.kubernetes.io/warn":                "privileged",
		"security.openshift.io/scc.podSecurityLabelSync": "false",
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   cnvNamespace,
			Labels: podSecurityLabels,
		},
	}

	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Namespace exists, update it to ensure Pod Security labels are set
			framework.Logf("Namespace %s already exists, updating Pod Security labels", cnvNamespace)
			existingNs, getErr := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, cnvNamespace, metav1.GetOptions{})
			if getErr != nil {
				return fmt.Errorf("failed to get existing namespace %s: %w", cnvNamespace, getErr)
			}
			if existingNs.Labels == nil {
				existingNs.Labels = make(map[string]string)
			}
			for k, v := range podSecurityLabels {
				existingNs.Labels[k] = v
			}
			_, updateErr := oc.AdminKubeClient().CoreV1().Namespaces().Update(ctx, existingNs, metav1.UpdateOptions{})
			if updateErr != nil {
				return fmt.Errorf("failed to update namespace %s with Pod Security labels: %w", cnvNamespace, updateErr)
			}
		} else {
			return fmt.Errorf("failed to create namespace %s: %w", cnvNamespace, err)
		}
	}

	// Step 2: Create OperatorGroup
	framework.Logf("Creating OperatorGroup %s", cnvOperatorGroup)
	operatorGroup := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operators.coreos.com/v1",
			"kind":       "OperatorGroup",
			"metadata": map[string]interface{}{
				"name":      cnvOperatorGroup,
				"namespace": cnvNamespace,
			},
			"spec": map[string]interface{}{
				"targetNamespaces": []interface{}{
					cnvNamespace,
				},
			},
		},
	}
	_, err = dynamicClient.Resource(operatorGroupGVR).Namespace(cnvNamespace).Create(ctx, operatorGroup, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create OperatorGroup: %w", err)
	}

	// Step 3: Create Subscription
	framework.Logf("Creating Subscription %s", cnvSubscription)
	subscription := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operators.coreos.com/v1alpha1",
			"kind":       "Subscription",
			"metadata": map[string]interface{}{
				"name":      cnvSubscription,
				"namespace": cnvNamespace,
			},
			"spec": map[string]interface{}{
				"channel":             "stable",
				"installPlanApproval": "Automatic",
				"name":                "kubevirt-hyperconverged",
				"source":              "redhat-operators",
				"sourceNamespace":     "openshift-marketplace",
				// Note: startingCSV can be specified for specific versions
				// "startingCSV":         "kubevirt-hyperconverged-operator.v4.17.0",
			},
		},
	}
	_, err = dynamicClient.Resource(subscriptionGVR).Namespace(cnvNamespace).Create(ctx, subscription, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create Subscription: %w", err)
	}

	// Step 4: Wait for CSV to be ready
	framework.Logf("Waiting for CNV operator to be installed...")
	err = waitForCNVOperatorReady(ctx, oc)
	if err != nil {
		return fmt.Errorf("CNV operator installation failed: %w", err)
	}

	// Step 5: Create HyperConverged CR
	framework.Logf("Creating HyperConverged CR %s", cnvHyperConverged)
	hyperConverged := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hco.kubevirt.io/v1beta1",
			"kind":       "HyperConverged",
			"metadata": map[string]interface{}{
				"name":      cnvHyperConverged,
				"namespace": cnvNamespace,
			},
			"spec": map[string]interface{}{
				"BareMetalPlatform": true,
				"infra":             map[string]interface{}{},
				"workloads":         map[string]interface{}{},
			},
		},
	}
	_, err = dynamicClient.Resource(hyperConvergedGVR).Namespace(cnvNamespace).Create(ctx, hyperConverged, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create HyperConverged CR: %w", err)
	}

	// Step 6: Wait for HyperConverged to be ready
	framework.Logf("Waiting for HyperConverged to be ready...")
	err = waitForHyperConvergedReady(ctx, oc)
	if err != nil {
		return fmt.Errorf("HyperConverged failed to become ready: %w", err)
	}

	// Step 7: Label worker nodes for CNV
	framework.Logf("Labeling worker nodes for CNV...")
	err = labelWorkerNodesForCNV(ctx, oc)
	if err != nil {
		framework.Logf("Warning: failed to label nodes for CNV: %v", err)
	}

	// Step 8: Wait for MCP rollout to complete (if any MachineConfigs were applied)
	framework.Logf("Checking MCP rollout status...")
	mcClient, err := machineconfigclient.NewForConfig(oc.AdminConfig())
	if err != nil {
		framework.Logf("Warning: failed to create MC client for MCP check: %v", err)
	} else {
		err = waitForMCP(ctx, mcClient, "worker", 30*time.Minute)
		if err != nil {
			framework.Logf("Warning: MCP rollout check failed: %v", err)
		}
	}

	framework.Logf("CNV operator installed successfully")
	return nil
}

// waitForCNVOperatorReady waits for the CNV operator CSV to be in Succeeded phase
func waitForCNVOperatorReady(ctx context.Context, oc *exutil.CLI) error {
	dynamicClient := oc.AdminDynamicClient()

	return wait.PollUntilContextTimeout(ctx, 15*time.Second, 15*time.Minute, true, func(ctx context.Context) (bool, error) {
		// List CSVs in the namespace
		csvList, err := dynamicClient.Resource(csvGVR).Namespace(cnvNamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			framework.Logf("Error listing CSVs: %v", err)
			return false, nil
		}

		for _, csv := range csvList.Items {
			name := csv.GetName()
			if strings.Contains(name, "kubevirt-hyperconverged") {
				phase, found, err := unstructured.NestedString(csv.Object, "status", "phase")
				if err != nil || !found {
					framework.Logf("CSV %s phase not found yet", name)
					return false, nil
				}
				framework.Logf("CSV %s phase: %s", name, phase)
				if phase == "Succeeded" {
					return true, nil
				}
			}
		}
		return false, nil
	})
}

// waitForHyperConvergedReady waits for the HyperConverged CR to be ready
func waitForHyperConvergedReady(ctx context.Context, oc *exutil.CLI) error {
	dynamicClient := oc.AdminDynamicClient()

	return wait.PollUntilContextTimeout(ctx, 15*time.Second, 20*time.Minute, true, func(ctx context.Context) (bool, error) {
		hc, err := dynamicClient.Resource(hyperConvergedGVR).Namespace(cnvNamespace).Get(ctx, cnvHyperConverged, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Error getting HyperConverged: %v", err)
			return false, nil
		}

		conditions, found, err := unstructured.NestedSlice(hc.Object, "status", "conditions")
		if err != nil || !found {
			framework.Logf("HyperConverged conditions not found yet")
			return false, nil
		}

		for _, cond := range conditions {
			condition, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _, _ := unstructured.NestedString(condition, "type")
			condStatus, _, _ := unstructured.NestedString(condition, "status")

			if condType == "Available" && condStatus == "True" {
				framework.Logf("HyperConverged is Available")
				return true, nil
			}
		}
		framework.Logf("Waiting for HyperConverged to become Available...")
		return false, nil
	})
}

// waitForMCP waits for a MachineConfigPool to be ready (not updating, updated, and all machines ready)
// Returns error immediately if the MCP becomes degraded
func waitForMCP(ctx context.Context, mcClient *machineconfigclient.Clientset, poolName string, timeout time.Duration) error {
	framework.Logf("Waiting for MCP %s to be ready (timeout: %v)...", poolName, timeout)

	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// Check conditions
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

		// Fail immediately if degraded
		if degraded {
			return false, fmt.Errorf("MachineConfigPool %s is degraded", poolName)
		}

		// Ready when not updating, updated condition is true, and pool has machines
		isReady := !updating && ready && mcp.Status.MachineCount > 0 && mcp.Status.ReadyMachineCount == mcp.Status.MachineCount

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

// labelWorkerNodesForCNV labels all worker nodes with kubevirt.io/schedulable=true
func labelWorkerNodesForCNV(ctx context.Context, oc *exutil.CLI) error {
	framework.Logf("Labeling worker nodes for CNV...")

	nodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
	if err != nil {
		return fmt.Errorf("failed to get worker nodes: %w", err)
	}

	for _, node := range nodes {
		framework.Logf("Labeling node %s with %s=true", node.Name, cnvNodeLabel)
		nodeCopy := node.DeepCopy()
		if nodeCopy.Labels == nil {
			nodeCopy.Labels = make(map[string]string)
		}
		nodeCopy.Labels[cnvNodeLabel] = "true"
		_, err := oc.AdminKubeClient().CoreV1().Nodes().Update(ctx, nodeCopy, metav1.UpdateOptions{})
		if err != nil {
			framework.Logf("Warning: failed to label node %s: %v", node.Name, err)
		}
	}

	return nil
}

// unlabelWorkerNodesForCNV removes the kubevirt.io/schedulable label from worker nodes
func unlabelWorkerNodesForCNV(ctx context.Context, oc *exutil.CLI) error {
	framework.Logf("Removing CNV labels from worker nodes...")

	nodes, err := getNodesByLabel(ctx, oc, cnvNodeLabel+"=true")
	if err != nil {
		return fmt.Errorf("failed to get CNV-labeled nodes: %w", err)
	}

	for _, node := range nodes {
		framework.Logf("Removing label %s from node %s", cnvNodeLabel, node.Name)
		nodeCopy := node.DeepCopy()
		delete(nodeCopy.Labels, cnvNodeLabel)
		_, err := oc.AdminKubeClient().CoreV1().Nodes().Update(ctx, nodeCopy, metav1.UpdateOptions{})
		if err != nil {
			framework.Logf("Warning: failed to unlabel node %s: %v", node.Name, err)
		}
	}

	return nil
}

// uninstallCNVOperator uninstalls the CNV operator and all related resources
func uninstallCNVOperator(ctx context.Context, oc *exutil.CLI) error {
	framework.Logf("Uninstalling CNV operator...")

	dynamicClient := oc.AdminDynamicClient()

	// Step 1: Delete HyperConverged CR
	framework.Logf("Deleting HyperConverged CR %s", cnvHyperConverged)
	err := dynamicClient.Resource(hyperConvergedGVR).Namespace(cnvNamespace).Delete(ctx, cnvHyperConverged, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		framework.Logf("Warning: failed to delete HyperConverged CR: %v", err)
	}

	// Wait for HyperConverged to be deleted
	framework.Logf("Waiting for HyperConverged CR to be deleted...")
	_ = wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := dynamicClient.Resource(hyperConvergedGVR).Namespace(cnvNamespace).Get(ctx, cnvHyperConverged, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		framework.Logf("Waiting for HyperConverged to be deleted...")
		return false, nil
	})

	// Step 2: Delete Subscription
	framework.Logf("Deleting Subscription %s", cnvSubscription)
	err = dynamicClient.Resource(subscriptionGVR).Namespace(cnvNamespace).Delete(ctx, cnvSubscription, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		framework.Logf("Warning: failed to delete Subscription: %v", err)
	}

	// Step 3: Delete all CSVs in the namespace
	framework.Logf("Deleting CSVs in namespace %s", cnvNamespace)
	csvList, err := dynamicClient.Resource(csvGVR).Namespace(cnvNamespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, csv := range csvList.Items {
			_ = dynamicClient.Resource(csvGVR).Namespace(cnvNamespace).Delete(ctx, csv.GetName(), metav1.DeleteOptions{})
		}
	}

	// Step 4: Delete OperatorGroup
	framework.Logf("Deleting OperatorGroup %s", cnvOperatorGroup)
	err = dynamicClient.Resource(operatorGroupGVR).Namespace(cnvNamespace).Delete(ctx, cnvOperatorGroup, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		framework.Logf("Warning: failed to delete OperatorGroup: %v", err)
	}

	// Step 5: Remove node labels
	framework.Logf("Removing CNV node labels...")
	_ = unlabelWorkerNodesForCNV(ctx, oc)

	// Step 6: Delete namespace
	framework.Logf("Deleting namespace %s", cnvNamespace)
	err = oc.AdminKubeClient().CoreV1().Namespaces().Delete(ctx, cnvNamespace, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		framework.Logf("Warning: failed to delete namespace: %v", err)
	}

	// Wait for namespace to be deleted
	framework.Logf("Waiting for namespace to be deleted...")
	_ = wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, cnvNamespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		framework.Logf("Waiting for namespace %s to be deleted...", cnvNamespace)
		return false, nil
	})

	// Step 7: Wait for MCP rollout to complete (if any MachineConfigs were removed)
	framework.Logf("Checking MCP rollout status after CNV uninstallation...")
	mcClient, err := machineconfigclient.NewForConfig(oc.AdminConfig())
	if err != nil {
		framework.Logf("Warning: failed to create MC client for MCP check: %v", err)
	} else {
		err = waitForMCP(ctx, mcClient, "worker", 30*time.Minute)
		if err != nil {
			framework.Logf("Warning: MCP rollout check failed: %v", err)
		}
	}

	framework.Logf("CNV operator uninstalled successfully")
	return nil
}

// ensureDropInDirectoryExists creates the drop-in directory on worker nodes if it doesn't exist
func ensureDropInDirectoryExists(ctx context.Context, oc *exutil.CLI, dirPath string) error {
	nodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
	if err != nil {
		return fmt.Errorf("failed to get worker nodes: %w", err)
	}

	for _, node := range nodes {
		_, err := ExecOnNodeWithChroot(oc, node.Name, "mkdir", "-p", dirPath)
		if err != nil {
			framework.Logf("Warning: failed to create directory on node %s: %v", node.Name, err)
		}
	}

	return nil
}

// GetReadySchedulableWorkerNodes returns ready schedulable worker nodes.
// This function filters out nodes with NoSchedule/NoExecute taints and non-worker nodes,
// making it suitable for tests that need to select worker nodes for workload placement.
func GetReadySchedulableWorkerNodes(ctx context.Context, client kubernetes.Interface) ([]v1.Node, error) {
	// Get ready schedulable nodes (excludes nodes with NoSchedule/NoExecute taints)
	nodes, err := e2enode.GetReadySchedulableNodes(ctx, client)
	if err != nil {
		return nil, err
	}

	// Filter for worker nodes only
	var workerNodes []v1.Node
	for _, node := range nodes.Items {
		if _, hasWorkerLabel := node.Labels["node-role.kubernetes.io/worker"]; hasWorkerLabel {
			workerNodes = append(workerNodes, node)
		}
	}

	return workerNodes, nil
}
