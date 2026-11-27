package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

// SkipOnMicroShift skips the current test if the cluster is MicroShift.
func SkipOnMicroShift(oc *exutil.CLI) {
	isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
	o.Expect(err).NotTo(o.HaveOccurred())
	if isMicroShift {
		g.Skip("Skipping test on MicroShift cluster")
	}
}

// SkipOnHyperShift skips the current test if the cluster is HyperShift (External topology).
func SkipOnHyperShift(ctx context.Context, oc *exutil.CLI) {
	isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
	o.Expect(err).NotTo(o.HaveOccurred())
	if isHyperShift {
		g.Skip("Skipping test on HyperShift cluster - MachineConfig API not available")
	}
}

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

// getPureWorkerNodesFromCluster returns worker nodes that are not also control plane nodes.
func getPureWorkerNodesFromCluster(ctx context.Context, oc *exutil.CLI) ([]corev1.Node, error) {
	workers, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
	if err != nil {
		return nil, err
	}
	pureWorkers := getPureWorkerNodes(workers)
	if len(pureWorkers) == 0 {
		return nil, fmt.Errorf("no pure worker nodes available")
	}
	return pureWorkers, nil
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
			framework.Logf("Skipping node %s (control-plane role)", node.Name)
			continue
		}
		if role, found := NodeHasCustomRole(node); found {
			framework.Logf("Skipping node %s (already has custom role %q)", node.Name, role)
			continue
		}
		pureWorkers = append(pureWorkers, node)
	}
	return pureWorkers
}

const (
	// DebugNamespace is the namespace for debug pods
	DebugNamespace = "openshift-machine-config-operator"
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

func execOnNodeWithDebug(ctx context.Context, oc *exutil.CLI, nodeName string, timeout time.Duration, args []string) (string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	execCmd, stdOutBuf, stdErrBuf, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args(args...).Background()
	if err != nil {
		return "", err
	}

	type result struct {
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		resultCh <- result{err: execCmd.Wait()}
	}()

	select {
	case res := <-resultCh:
		stdOut := strings.TrimSpace(stdOutBuf.String())
		stdErr := strings.TrimSpace(stdErrBuf.String())
		if res.err != nil {
			return stdOut, fmt.Errorf("oc debug failed: %w\nStdErr: %s", res.err, stdErr)
		}
		return stdOut, nil
	case <-timeoutCtx.Done():
		var killErr error
		if execCmd.Process != nil {
			killErr = execCmd.Process.Kill()
		}
		if ctx.Err() != nil {
			return "", fmt.Errorf("oc debug command canceled on node %s: %w", nodeName, ctx.Err())
		}
		if killErr != nil {
			return "", fmt.Errorf("oc debug command timed out after %v on node %s; failed to stop debug process: %w", timeout, nodeName, killErr)
		}
		return "", fmt.Errorf("oc debug command timed out after %v on node %s (cleanup likely hung)", timeout, nodeName)
	}
}

func ExecOnNodeWithChroot(ctx context.Context, oc *exutil.CLI, nodeName string, cmd ...string) (string, error) {
	args := append([]string{"node/" + nodeName, "-n" + DebugNamespace, "--", "chroot", "/host"}, cmd...)
	return execOnNodeWithDebug(ctx, oc, nodeName, 2*time.Minute, args)
}

func ExecOnNodeWithNsenter(ctx context.Context, oc *exutil.CLI, nodeName string, cmd ...string) (string, error) {
	nsenterCmd := append([]string{"nsenter", "-a", "-t", "1"}, cmd...)
	args := append([]string{"node/" + nodeName, "-n" + DebugNamespace, "--"}, nsenterCmd...)
	return execOnNodeWithDebug(ctx, oc, nodeName, 2*time.Minute, args)
}

// createDropInFile creates a drop-in configuration file on the specified node
func createDropInFile(ctx context.Context, oc *exutil.CLI, nodeName, filePath, content string) error {
	escapedContent := strings.ReplaceAll(content, "'", "'\\''")
	cmd := fmt.Sprintf("echo '%s' > %s && chmod 644 %s", escapedContent, filePath, filePath)
	_, err := ExecOnNodeWithChroot(ctx, oc, nodeName, "sh", "-c", cmd)
	return err
}

// removeDropInFile removes a drop-in configuration file from the specified node
func removeDropInFile(ctx context.Context, oc *exutil.CLI, nodeName, filePath string) error {
	_, err := ExecOnNodeWithChroot(ctx, oc, nodeName, "rm", "-f", filePath)
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
// Retries on transient network errors which are common on real clusters
func restartKubeletOnNode(ctx context.Context, oc *exutil.CLI, nodeName string) error {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		_, err := ExecOnNodeWithChroot(ctx, oc, nodeName, "systemctl", "restart", "kubelet")
		if err == nil {
			return nil
		}
		lastErr = err
		if !isTransientNetworkError(err) {
			return fmt.Errorf("failed to restart kubelet on %s: %w", nodeName, err)
		}
		if attempt == maxAttempts-1 {
			break
		}
		backoff := time.Duration((attempt+1)*5) * time.Second
		framework.Logf("Attempt %d/%d to restart kubelet on %s failed: %v; retrying in %s",
			attempt+1, maxAttempts, nodeName, err, backoff)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("context canceled while restarting kubelet on %s: %w", nodeName, ctx.Err())
		case <-timer.C:
		}
	}
	return fmt.Errorf("failed to restart kubelet on %s after %d attempts: %w", nodeName, maxAttempts, lastErr)
}

// isTransientNetworkError checks if the error is a transient network error worth retrying
func isTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	transientErrors := []string{
		"connection refused",
		"connection reset",
		"connection timed out",
		"i/o timeout",
	}
	for _, transientErr := range transientErrors {
		if strings.Contains(errStr, transientErr) {
			return true
		}
	}
	return false
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
	removeDropInFile(ctx, oc, nodeName, filePath)
	framework.Logf("Restarting kubelet on node: %s", nodeName)
	restartKubeletOnNode(ctx, oc, nodeName)
	framework.Logf("Waiting for node to be ready...")
	waitForNodeToBeReady(ctx, oc, nodeName)
}

// ============================================================================
// CNV Operator Installation/Uninstallation Functions
// ============================================================================

// isCNVInstalled checks if CNV operator is installed. Lookup errors are returned
// instead of being treated as "not installed", so a transient API failure cannot send
// the test down the install path and tear down an installation it did not create.
func isCNVInstalled(ctx context.Context, oc *exutil.CLI) (bool, error) {
	// Check if CNV namespace exists
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, cnvNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to get namespace %s: %w", cnvNamespace, err)
	}

	// Check if HyperConverged CR exists
	dynamicClient := oc.AdminDynamicClient()
	_, err = dynamicClient.Resource(hyperConvergedGVR).Namespace(cnvNamespace).Get(ctx, cnvHyperConverged, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to get HyperConverged %s/%s: %w", cnvNamespace, cnvHyperConverged, err)
	}
	return true, nil
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
		return fmt.Errorf("failed to create MC client for MCP check: %w", err)
	}

	machineCount, err := workerPoolMachineCount(ctx, mcClient)
	if err != nil {
		return fmt.Errorf("failed to check worker MachineConfigPool: %w", err)
	}
	if machineCount == 0 {
		framework.Logf("Worker MachineConfigPool has no machines, skipping the rollout wait")
	} else if err = WaitForMCP(ctx, mcClient, "worker", workerRolloutTimeout(machineCount)); err != nil {
		return fmt.Errorf("MCP rollout failed after CNV installation: %w", err)
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

	return wait.PollUntilContextTimeout(ctx, 15*time.Second, 15*time.Minute, true, func(ctx context.Context) (bool, error) {
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

type waitMCPOptions struct {
	machineCount  *int32
	allowDegraded bool
}

// WaitMCPWithMachineCount waits until the pool has the exact machine count.
func WaitMCPWithMachineCount(count int32) func(*waitMCPOptions) {
	return func(o *waitMCPOptions) {
		o.machineCount = &count
	}
}

// WaitMCPAllowDegraded tolerates transient Degraded/RenderDegraded during polling instead of
// failing immediately. The pool must still become fully healthy (!degraded, !renderDegraded,
// updated, all machines ready) before this wait succeeds.
func WaitMCPAllowDegraded() func(*waitMCPOptions) {
	return func(o *waitMCPOptions) {
		o.allowDegraded = true
	}
}

// workerPoolMachineCount returns the number of machines in the worker MachineConfigPool.
// It is 0 on single-node topologies, where the only node belongs to master and
// WaitForMCP (which requires MachineCount > 0) would never succeed.
func workerPoolMachineCount(ctx context.Context, mcClient *machineconfigclient.Clientset) (int32, error) {
	mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, "worker", metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to get worker MachineConfigPool: %w", err)
	}
	return mcp.Status.MachineCount, nil
}

// workerRolloutTimeout scales the worker pool rollout budget with the pool size: on
// AWS each worker takes roughly 2m20s to drain and reboot, so a fixed 15m is right at
// the edge for a 6-node pool.
func workerRolloutTimeout(machineCount int32) time.Duration {
	return max(15*time.Minute, time.Duration(machineCount)*5*time.Minute)
}

// WaitForMCP waits for a MachineConfigPool to be ready (not updating, updated, and all machines ready).
// By default it returns an error immediately if the MCP becomes degraded.
func WaitForMCP(ctx context.Context, mcClient *machineconfigclient.Clientset, poolName string, timeout time.Duration, opts ...func(*waitMCPOptions)) error {
	options := waitMCPOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	framework.Logf("Waiting for MCP %s to be ready (timeout: %v)...", poolName, timeout)

	poolSeen := false
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				// Only treat NotFound as success when draining a pool we have previously observed.
				// A typo in poolName must not pass silently on the first poll.
				if poolSeen && options.machineCount != nil && *options.machineCount == 0 {
					framework.Logf("MachineConfigPool %s no longer exists after draining to 0 machines", poolName)
					return true, nil
				}
			}
			return false, err
		}
		poolSeen = true

		updating := false
		degraded := false
		renderDegraded := false
		ready := false

		for _, condition := range mcp.Status.Conditions {
			switch condition.Type {
			case machineconfigv1.MachineConfigPoolUpdating:
				if condition.Status == corev1.ConditionTrue {
					updating = true
				}
			case machineconfigv1.MachineConfigPoolDegraded:
				if condition.Status == corev1.ConditionTrue {
					degraded = true
				}
			case machineconfigv1.MachineConfigPoolRenderDegraded:
				if condition.Status == corev1.ConditionTrue {
					renderDegraded = true
				}
			case machineconfigv1.MachineConfigPoolUpdated:
				if condition.Status == corev1.ConditionTrue {
					ready = true
				}
			}
		}

		if !options.allowDegraded {
			if degraded {
				return false, fmt.Errorf("MachineConfigPool %s is degraded", poolName)
			}
			if renderDegraded {
				return false, fmt.Errorf("MachineConfigPool %s render is degraded", poolName)
			}
		}

		// Drained pool: all nodes removed and conditions stable before cleanup continues.
		if options.machineCount != nil && *options.machineCount == 0 {
			isDrained := mcp.Status.MachineCount == 0 &&
				mcp.Status.ReadyMachineCount == 0 &&
				!updating && !degraded && !renderDegraded
			if isDrained {
				framework.Logf("MachineConfigPool %s drained: 0 machines, not updating/degraded", poolName)
				return true, nil
			}
			framework.Logf("MachineConfigPool %s waiting to drain: updating=%v degraded=%v renderDegraded=%v machines=%d/%d",
				poolName, updating, degraded, renderDegraded, mcp.Status.ReadyMachineCount, mcp.Status.MachineCount)
			return false, nil
		}

		isReady := !updating && !degraded && !renderDegraded && ready &&
			mcp.Status.MachineCount > 0 && mcp.Status.ReadyMachineCount == mcp.Status.MachineCount
		if options.machineCount != nil {
			isReady = isReady && mcp.Status.MachineCount == *options.machineCount
		}

		if isReady {
			framework.Logf("MachineConfigPool %s is ready: %d/%d machines ready",
				poolName, mcp.Status.ReadyMachineCount, mcp.Status.MachineCount)
		} else {
			framework.Logf("MachineConfigPool %s not ready yet: updating=%v degraded=%v renderDegraded=%v updated=%v machines=%d/%d",
				poolName, updating, degraded, renderDegraded, ready, mcp.Status.ReadyMachineCount, mcp.Status.MachineCount)
		}

		return isReady, nil
	})
}

// getWorkerGeneratedKubeletMC finds and returns the highest numbered worker-generated-kubelet MachineConfig.
// KubeletConfig changes affect the highest numbered config, so we return that one.
func getWorkerGeneratedKubeletMC(ctx context.Context, mcClient *machineconfigclient.Clientset) (*machineconfigv1.MachineConfig, error) {
	mcList, err := mcClient.MachineconfigurationV1().MachineConfigs().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var highestMC *machineconfigv1.MachineConfig
	for i := range mcList.Items {
		if strings.Contains(mcList.Items[i].Name, "worker-generated-kubelet") {
			if highestMC == nil || mcList.Items[i].Name > highestMC.Name {
				highestMC = &mcList.Items[i]
			}
		}
	}

	if highestMC == nil {
		return nil, fmt.Errorf("worker-generated-kubelet MachineConfig not found")
	}

	return highestMC, nil
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

// cnvAutopilotManagedBySelector selects the MachineConfigs created by CNV's
// virt-platform-autopilot (swap drop-in, PSI karg).
const cnvAutopilotManagedBySelector = "platform.kubevirt.io/managed-by=virt-platform-autopilot"

// removeCNVMachineConfigs deletes the MachineConfigs owned by CNV's
// virt-platform-autopilot. They are cluster-scoped with no ownerReference and CNV
// never deletes them, so uninstalling CNV would otherwise leave the
// /etc/openshift/kubelet.conf.d/90-swap.conf drop-in (swapBehavior=LimitedSwap)
// and the psi=1 kernel argument on every worker.
// Returns whether at least one MachineConfig was deleted. Deletion can be partial
// (the autopilot recreates one while its namespace is still terminating), but any
// deletion can re-render the worker config, so the caller syncs the rollout whenever
// this returns true, even alongside an error.
func removeCNVMachineConfigs(ctx context.Context, mcClient *machineconfigclient.Clientset) (bool, error) {
	mcV1 := mcClient.MachineconfigurationV1()
	deletedAny := false

	// The autopilot recreates its MachineConfigs while it is still running, so keep
	// deleting until the namespace teardown has stopped it for good.
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		mcs, err := mcV1.MachineConfigs().List(ctx, metav1.ListOptions{LabelSelector: cnvAutopilotManagedBySelector})
		if err != nil {
			return false, err
		}
		if len(mcs.Items) == 0 {
			return true, nil
		}

		for _, mc := range mcs.Items {
			framework.Logf("Deleting CNV MachineConfig %s", mc.Name)
		}
		deletedAny = true
		if err := mcV1.MachineConfigs().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: cnvAutopilotManagedBySelector}); err != nil {
			return false, fmt.Errorf("failed to delete CNV MachineConfigs: %w", err)
		}
		return false, nil
	})
	if err != nil {
		return deletedAny, fmt.Errorf("CNV autopilot MachineConfigs still present after CNV uninstall: %w", err)
	}
	return deletedAny, nil
}

// cleanupCNVMachineConfigs deletes any MachineConfigs created by CNV's
// virt-platform-autopilot and waits for the worker pool to roll the change out, so
// the swap drop-in and PSI karg it owns are gone from the nodes.
func cleanupCNVMachineConfigs(ctx context.Context, oc *exutil.CLI) error {
	mcClient, err := machineconfigclient.NewForConfig(oc.AdminConfig())
	if err != nil {
		return fmt.Errorf("failed to create MachineConfig client: %w", err)
	}

	// Even when nothing was deleted, the worker pool may still be rolling out a config
	// that carried the CNV content (an earlier cleanup may have stopped mid-rollout), so
	// always converge on a rendered config without it.
	deletedAny, removeErr := removeCNVMachineConfigs(ctx, mcClient)
	var errs []error
	if removeErr != nil {
		errs = append(errs, fmt.Errorf("failed to remove CNV autopilot MachineConfigs: %w", removeErr))
	} else {
		framework.Logf("Deleted CNV autopilot MachineConfigs: %t", deletedAny)
	}

	// WaitForMCP is deliberately not used here: it only reads pool conditions, so right
	// after the delete it still sees the stale Updated=True and returns, and a pool that
	// degrades mid-rollout would only surface as a generic timeout. The settle check
	// verifies the rendered config and reports a degraded pool itself.
	machineCount, err := workerPoolMachineCount(ctx, mcClient)
	if err != nil {
		errs = append(errs, err)
		return errors.Join(errs...)
	}
	if machineCount == 0 {
		framework.Logf("Worker MachineConfigPool has no machines, skipping the rollout wait")
		return errors.Join(errs...)
	}
	if err := waitForWorkerConfigWithoutCNV(ctx, mcClient, workerRolloutTimeout(machineCount)); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// waitForWorkerConfigWithoutCNV waits until the worker pool is stable on a rendered
// config that no longer carries CNV autopilot's content. The rendered config name can
// stay the same when the deleted MachineConfigs were never rendered, so this checks
// the content instead of the name.
func waitForWorkerConfigWithoutCNV(ctx context.Context, mcClient *machineconfigclient.Clientset, timeout time.Duration) error {
	mcV1 := mcClient.MachineconfigurationV1()
	stableChecks := 0

	framework.Logf("Waiting for the worker pool to settle without the CNV MachineConfigs...")
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		mcp, err := mcV1.MachineConfigPools().Get(ctx, "worker", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if machineConfigPoolConditionTrue(mcp, machineconfigv1.MachineConfigPoolDegraded) ||
			machineConfigPoolConditionTrue(mcp, machineconfigv1.MachineConfigPoolRenderDegraded) {
			return false, fmt.Errorf("worker MachineConfigPool is degraded while waiting for the CNV MachineConfig removal (spec=%s status=%s updated=%d/%d)",
				mcp.Spec.Configuration.Name, mcp.Status.Configuration.Name, mcp.Status.UpdatedMachineCount, mcp.Status.MachineCount)
		}
		framework.Logf("Worker pool settling: spec=%s status=%s updated=%d/%d",
			mcp.Spec.Configuration.Name, mcp.Status.Configuration.Name, mcp.Status.UpdatedMachineCount, mcp.Status.MachineCount)
		if mcp.Spec.Configuration.Name != mcp.Status.Configuration.Name || !machineConfigPoolConditionTrue(mcp, machineconfigv1.MachineConfigPoolUpdated) {
			stableChecks = 0
			return false, nil
		}

		rendered, err := mcV1.MachineConfigs().Get(ctx, mcp.Status.Configuration.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if workerConfigHasCNVSwapDropIn(rendered) {
			framework.Logf("Rendered worker config %s still contains the CNV swap drop-in", mcp.Status.Configuration.Name)
			stableChecks = 0
			return false, nil
		}

		// Require two consecutive stable checks so a render built from a stale source
		// set cannot slip in between the pool and rendered-config reads.
		stableChecks++
		return stableChecks >= 2, nil
	})
	if err != nil {
		return fmt.Errorf("worker pool never settled on a config without the CNV MachineConfigs: %w", err)
	}
	return nil
}

// workerConfigHasCNVSwapDropIn reports whether a rendered MachineConfig still carries
// the swap kubelet drop-in written by CNV's autopilot. The rendered config merges every
// MachineConfig in the pool, so this only checks the CNV-specific path; the PSI karg is
// covered by removeCNVMachineConfigs leaving no autopilot-labeled MachineConfig behind.
func workerConfigHasCNVSwapDropIn(mc *machineconfigv1.MachineConfig) bool {
	if len(mc.Spec.Config.Raw) == 0 {
		return false
	}
	var cfg struct {
		Storage struct {
			Files []struct {
				Path string `json:"path"`
			} `json:"files"`
		} `json:"storage"`
	}
	if err := json.Unmarshal(mc.Spec.Config.Raw, &cfg); err != nil {
		framework.Logf("Warning: failed to parse rendered MachineConfig %s: %v", mc.Name, err)
		return true
	}
	for _, file := range cfg.Storage.Files {
		if file.Path == cnvDropInDir+"/90-swap.conf" {
			return true
		}
	}
	return false
}

// machineConfigPoolConditionTrue reports whether the pool condition has status True.
func machineConfigPoolConditionTrue(mcp *machineconfigv1.MachineConfigPool, conditionType machineconfigv1.MachineConfigPoolConditionType) bool {
	for _, condition := range mcp.Status.Conditions {
		if condition.Type == conditionType {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// uninstallCNVOperator uninstalls the CNV operator and all related resources
func uninstallCNVOperator(ctx context.Context, oc *exutil.CLI) error {
	framework.Logf("Uninstalling CNV operator...")

	dynamicClient := oc.AdminDynamicClient()

	// Step 1: Delete HyperConverged CR
	// Give the operator 3 minutes to clean up gracefully; if the CR is still stuck,
	// strip its finalizers so deletion can proceed.
	framework.Logf("Deleting HyperConverged CR %s", cnvHyperConverged)
	err := dynamicClient.Resource(hyperConvergedGVR).Namespace(cnvNamespace).Delete(ctx, cnvHyperConverged, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		framework.Logf("Warning: failed to delete HyperConverged CR: %v", err)
	}

	if err == nil || !apierrors.IsNotFound(err) {
		framework.Logf("Waiting for HyperConverged CR to be deleted...")
		finalizersStripped := false
		startTime := time.Now()
		_ = wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
			_, getErr := dynamicClient.Resource(hyperConvergedGVR).Namespace(cnvNamespace).Get(ctx, cnvHyperConverged, metav1.GetOptions{})
			if apierrors.IsNotFound(getErr) {
				return true, nil
			}

			if !finalizersStripped && time.Since(startTime) > 3*time.Minute {
				framework.Logf("HyperConverged CR still present after 3m, stripping finalizers")
				patch := []byte(`{"metadata":{"finalizers":[]}}`)
				_, patchErr := dynamicClient.Resource(hyperConvergedGVR).Namespace(cnvNamespace).Patch(
					ctx, cnvHyperConverged, types.MergePatchType, patch, metav1.PatchOptions{})
				if patchErr != nil && !apierrors.IsNotFound(patchErr) {
					framework.Logf("Warning: failed to strip finalizers from HyperConverged CR: %v", patchErr)
				}
				finalizersStripped = true
			}

			framework.Logf("Waiting for HyperConverged to be deleted...")
			return false, nil
		})
	}

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

	// Wait for namespace to be deleted. While it exists the autopilot can recreate its
	// MachineConfigs, so a timeout here must fail the cleanup instead of reporting success.
	framework.Logf("Waiting for namespace to be deleted...")
	var cleanupErrs []error
	err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, cnvNamespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		framework.Logf("Waiting for namespace %s to be deleted...", cnvNamespace)
		return false, nil
	})
	if err != nil {
		cleanupErrs = append(cleanupErrs, fmt.Errorf("namespace %s was not deleted, CNV autopilot may still be running: %w", cnvNamespace, err))
	}

	// Step 7: Delete the MachineConfigs CNV's virt-platform-autopilot created and wait
	// for the worker pool to roll the change out. Without this they outlive CNV and keep
	// overriding swapBehavior=NoSwap on every worker.
	framework.Logf("Removing CNV-managed MachineConfigs...")
	if err := cleanupCNVMachineConfigs(ctx, oc); err != nil {
		cleanupErrs = append(cleanupErrs, err)
	}

	if err := waitForKubevirtClusterCleanup(ctx, oc); err != nil {
		cleanupErrs = append(cleanupErrs, fmt.Errorf("kubevirt cluster cleanup incomplete after CNV uninstall: %w", err))
	}
	if len(cleanupErrs) > 0 {
		return errors.Join(cleanupErrs...)
	}

	framework.Logf("CNV operator uninstalled successfully")
	return nil
}

var kubevirtLeakClusterRoleBindingPrefixes = []string{
	"kubevirt.io:",
	"cdi.kubevirt.io:",
	"instancetype.kubevirt.io:",
}

func kubevirtClusterLeaks(ctx context.Context, oc *exutil.CLI) string {
	crbs, err := oc.AdminKubeClient().RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Sprintf("list ClusterRoleBindings: %v", err)
	}
	for _, crb := range crbs.Items {
		for _, prefix := range kubevirtLeakClusterRoleBindingPrefixes {
			if strings.HasPrefix(crb.Name, prefix) {
				return fmt.Sprintf("ClusterRoleBinding %q still present", crb.Name)
			}
		}
	}
	return ""
}

func waitForKubevirtClusterCleanup(ctx context.Context, oc *exutil.CLI) error {
	framework.Logf("Waiting for kubevirt cluster RBAC to be removed after CNV uninstall...")
	err := wait.PollUntilContextTimeout(ctx, 15*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		if leak := kubevirtClusterLeaks(ctx, oc); leak != "" {
			framework.Logf("Still waiting: %s", leak)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		if leak := kubevirtClusterLeaks(ctx, oc); leak != "" {
			return fmt.Errorf("%s", leak)
		}
		return err
	}
	return nil
}

// ensureDropInDirectoryExists creates the drop-in directory on worker nodes if it doesn't exist
func ensureDropInDirectoryExists(ctx context.Context, oc *exutil.CLI, dirPath string) error {
	nodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
	if err != nil {
		return fmt.Errorf("failed to get worker nodes: %w", err)
	}

	for _, node := range nodes {
		_, err := ExecOnNodeWithChroot(ctx, oc, node.Name, "mkdir", "-p", dirPath)
		if err != nil {
			framework.Logf("Warning: failed to create directory on node %s: %v", node.Name, err)
		}
	}

	return nil
}

// GetFirstReadyWorkerNode returns the name of the first Ready worker node in the cluster.
func GetFirstReadyWorkerNode(oc *exutil.CLI) string {
	ctx := context.Background()
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/worker",
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(nodes.Items).NotTo(o.BeEmpty(), "no worker nodes found")

	for _, node := range nodes.Items {
		if role, found := NodeHasCustomRole(node); found {
			framework.Logf("Skipping node %s (already has custom role %q)", node.Name, role)
			continue
		}
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				return node.Name
			}
		}
	}
	o.Expect(false).To(o.BeTrue(), "no Ready worker node without custom roles found")
	return ""
}

// GetPodNetNs retrieves the network namespace path for a pod using crictl.
// It uses crictl to get the sandbox ID and then inspects it to extract the NetNS path.
// Returns the NetNS path and an error if not found.
func GetPodNetNs(ctx context.Context, oc *exutil.CLI, nodeName, podName string) (string, error) {
	// Get sandbox ID using crictl
	sandboxID, err := ExecOnNodeWithChroot(ctx, oc, nodeName, "crictl", "pods", "--name", podName, "-q")
	if err != nil || sandboxID == "" {
		framework.Logf("Failed to get sandbox ID for pod %s: %v", podName, err)
		return "", fmt.Errorf("failed to get sandbox ID for pod %s: %w", podName, err)
	}
	sandboxID = strings.TrimSpace(sandboxID)
	framework.Logf("Found sandbox ID: %s", sandboxID)

	// Extract network namespace path from sandbox inspection
	netNsStr, err := ExecOnNodeWithChroot(ctx, oc, nodeName, "sh", "-c", fmt.Sprintf("crictl inspectp %s | grep -i netns", sandboxID))
	if err != nil {
		framework.Logf("Failed to get NetNS from crictl inspect: %v", err)
		return "", fmt.Errorf("failed to get NetNS from crictl inspect: %w", err)
	}

	// Extract NetNS path from the output (format: "linux": { "namespaces": [ { "type": "network", "path": "/var/run/netns/..." } ] } )
	re := regexp.MustCompile(`"path":\s*"([^"]+)"`)
	matches := re.FindStringSubmatch(netNsStr)
	if len(matches) < 2 {
		framework.Logf("NetNS path not found in crictl output for pod %s", podName)
		return "", fmt.Errorf("NetNS path not found in crictl output for pod %s", podName)
	}
	netNsPath := matches[1]
	framework.Logf("Extracted NetNS path: %v", netNsPath)
	return netNsPath, nil
}

// CheckNetNsCleaned verifies that the network namespace file has been cleaned up.
// It checks if the NetNS path no longer exists on the node.
// Returns nil if the file is cleaned, error if it still exists.
func CheckNetNsCleaned(ctx context.Context, oc *exutil.CLI, nodeName, netNsPath string) error {
	// Use test command which returns proper exit code
	_, err := ExecOnNodeWithChroot(ctx, oc, nodeName, "test", "-e", netNsPath)
	if err != nil {
		// Non-nil err: file absent (test exit 1) OR exec/debug failure.
		framework.Logf("NetNS file considered cleaned (test -e returned error: %v)", err)
		return nil
	}
	// No error means file still exists
	return fmt.Errorf("NetNS file still exists at %s", netNsPath)
}

func GetNotReadyNodes(ctx context.Context, oc *exutil.CLI) ([]string, error) {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var notReadyNodes []string
	for _, node := range nodes.Items {
		if !isNodeInReadyState(&node) {
			notReadyNodes = append(notReadyNodes, node.Name)
		}
	}

	return notReadyNodes, nil
}

func EnsureNodesReady(ctx context.Context, oc *exutil.CLI) {
	notReadyNodes, err := GetNotReadyNodes(ctx, oc)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to check node readiness")

	// On intentionally degraded TNF clusters (DEGRADED_NODE=true), one control-plane
	// node is expected to be shut down. Tolerate exactly one NotReady node so tests
	// can run against the surviving single-node cluster.
	if exutil.ClusterDegraded && len(notReadyNodes) == 1 && exutil.IsTwoNodeFencing(ctx, oc.AdminConfigClient()) {
		framework.Logf("Tolerating one NotReady node %v on intentionally degraded TNF cluster", notReadyNodes)
		notReadyNodes = nil
	}

	o.Expect(notReadyNodes).To(o.BeEmpty(),
		"Cannot start test: nodes not Ready: %v. Cluster may be recovering from previous test.", notReadyNodes)
}

// EnsureNodeResourceNodesReady checks only nodes reserved for this test label.
// Unlike EnsureNodesReady, parallel NodeResource tests on other pool nodes
// may temporarily leave a node NotReady without blocking this test.
func EnsureNodeResourceNodesReady(ctx context.Context, oc *exutil.CLI, label string) {
	nodes, err := GetNodeResourceNodes(ctx, oc, label)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get NodeResource nodes for label %s", label)

	var notReady []string
	for _, node := range nodes {
		if !isNodeInReadyState(&node) {
			notReady = append(notReady, node.Name)
		}
	}

	o.Expect(notReady).To(o.BeEmpty(),
		"Cannot start test: NodeResource node(s) not Ready for label %s: %v. Assigned node(s) may still be recovering from this test's own setup.", label, notReady)
}

func CreatePodAndWaitForRunning(ctx context.Context, oc *exutil.CLI, pod *corev1.Pod) *corev1.Pod {
	_, err := oc.AdminKubeClient().CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	err = e2epod.WaitForPodRunningInNamespace(ctx, oc.AdminKubeClient(), pod)
	o.Expect(err).NotTo(o.HaveOccurred())

	created, err := oc.AdminKubeClient().CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	return created
}

func createDirectoriesOnNode(oc *exutil.CLI, nodeName string, dirs []string) error {
	args := append([]string{"mkdir", "-p"}, dirs...)
	_, err := ExecOnNodeWithChroot(context.Background(), oc, nodeName, args...)
	if err != nil {
		return fmt.Errorf("failed to create directories %v on node %s: %v", dirs, nodeName, err)
	}
	framework.Logf("Node %s: directories created: %v", nodeName, dirs)
	return nil
}

func cleanupDirectoriesOnNode(oc *exutil.CLI, nodeName string, dirs []string) error {
	args := append([]string{"rm", "-rf"}, dirs...)
	_, err := ExecOnNodeWithChroot(context.Background(), oc, nodeName, args...)
	if err != nil {
		return fmt.Errorf("failed to cleanup directories %v on node %s: %w", dirs, nodeName, err)
	}
	return nil
}

// NodeResourceLabelKey is the label key used by the NodeResource scheduler
// to mark worker nodes as assigned to a specific test.
const NodeResourceLabelKey = "noderesource.test.openshift.io/name"

// GetNodeResourceNodes returns worker nodes assigned to a NodeResource test by label.
// Tests declare their requirements via g.Label("NodeResource", "NodeResourceNumNodes=N",
// "NodeResourceName=X") and should use this function to find their assigned nodes.
func GetNodeResourceNodes(ctx context.Context, oc *exutil.CLI, label string) ([]corev1.Node, error) {
	labelSelector := fmt.Sprintf("%s=%s", NodeResourceLabelKey, label)
	nodes, err := getNodesByLabel(ctx, oc, labelSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to get NodeResource nodes with label %s: %w", label, err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes found with NodeResource label %s", label)
	}
	return nodes, nil
}

// GetNodeResource returns the name of the first node assigned to a NodeResource test.
func GetNodeResource(ctx context.Context, oc *exutil.CLI, label string) (string, error) {
	nodes, err := GetNodeResourceNodes(ctx, oc, label)
	if err != nil {
		return "", err
	}
	return nodes[0].Name, nil
}
