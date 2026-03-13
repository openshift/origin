package two_node

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	v1 "github.com/openshift/api/config/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	"github.com/openshift/origin/test/extended/baremetal"
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/apis"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	extraWorkerProvisioningTimeout = 15 * time.Minute
)

// initialState holds the cluster state recorded before the test begins,
// so that post-test assertions can compare against the baseline.
type initialState struct {
	nodes          *corev1.NodeList
	nodeCount      int
	mcoClient      machineconfigclient.Interface
	workerMCPCount int32
}

var bmhGVR = schema.GroupVersionResource{
	Group: "metal3.io", Resource: "baremetalhosts", Version: "v1alpha1",
}

var _ = g.Describe("[sig-node][apigroup:config.openshift.io][OCPFeatureGate:HighlyAvailableArbiter][Serial] Extra worker scaling in HighlyAvailableArbiterMode", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("").AsAdmin()
	var helper *baremetal.BaremetalTestHelper

	g.BeforeEach(func() {
		g.By("SETUP: Validating cluster topology is HighlyAvailableArbiter")
		utils.SkipIfNotTopology(oc, v1.HighlyAvailableArbiterMode)

		// We intentionally do NOT call helper.Setup() here: its internal waitForDeletion
		// has a hardcoded 5-minute timeout with o.Expect, which fails the test if a stale
		// BMH from a previous (killed) run is still deprovisioning. Instead, we use our
		// own cleanup with a 30-minute timeout that logs instead of failing.
		g.By("SETUP: Cleaning up stale extra worker BMHs from previous runs")
		bmhClient := oc.AdminDynamicClient().Resource(bmhGVR).Namespace(machineAPINamespace)
		deleteExtraWorkerBMHs(bmhClient)

		g.By("SETUP: Initializing baremetal test helper")
		helper = baremetal.NewBaremetalTestHelper(oc.AdminDynamicClient())

		g.By("SETUP: Ensuring extra worker data is available")
		if !helper.CanDeployExtraWorkers() {
			g.Skip("Extra worker data not available - deploy with NUM_EXTRA_WORKERS=1 in dev-scripts config")
		}
	})

	g.AfterEach(func() {
		if helper == nil {
			return
		}
		// Do NOT call helper.DeleteAllExtraWorkers() here: its internal waitForDeletion has a
		// hardcoded 5-minute timeout, which is insufficient for a provisioned BMH — Metal3 must
		// complete a full deprovisioning cycle (power-off + disk clean via Redfish) before the
		// object is removed, which routinely takes >5 minutes.
		//
		// Instead, we delete only the extraworker BMHs (by name prefix) and wait up to 30 minutes.
		g.By("CLEANUP: Deleting extra worker BMHs and waiting for Metal3 deprovisioning")
		bmhClient := oc.AdminDynamicClient().Resource(bmhGVR).Namespace(machineAPINamespace)
		deleteExtraWorkerBMHs(bmhClient)

		g.By("CLEANUP: Removing stale extra worker Node objects from the cluster")
		deleteExtraWorkerNodes(oc)
	})

	g.It("should deploy an extra worker node that joins the cluster and becomes Ready [Timeout:60m][Slow]", func() {
		g.By("STEP-01: Recording initial values for the test")
		state := recordInitialState(oc)

		g.By("STEP-02: Provisioning extra worker BMH")
		provisionExtraWorker(oc, helper)

		g.By("STEP-03: Approving pending CSRs for the new worker")
		approveWorkerCSRs(oc)

		g.By("STEP-04: Waiting for a new worker node to become Ready")
		newWorkerName := waitForNewWorkerReady(oc, state.nodes)

		g.By("STEP-05: Verifying new node membership and worker label")
		newNode := verifyNodeMembership(oc, newWorkerName, state.nodeCount)

		g.By("STEP-06: Verifying the new node has no pressure conditions")
		verifyNodeHealth(newNode)

		g.By("STEP-07: Verifying a pod can be scheduled and run on the new node")
		verifyPodSchedulable(oc, newWorkerName)

		g.By("STEP-08: Verifying the worker MachineConfigPool converges with the new node")
		waitForWorkerMCPConvergence(state.mcoClient, state.workerMCPCount)

		g.By("STEP-09: Verifying the new node's kubelet version matches existing nodes")
		verifyKubeletVersion(oc, newNode)

		e2e.Logf("Extra worker %s joined the cluster successfully", newWorkerName)
	})
})

// --- Test step helpers ---

func recordInitialState(oc *exutil.CLI) initialState {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	mcoClient := machineconfigclient.NewForConfigOrDie(oc.AdminConfig())
	mcpCount := getWorkerMCPMachineCount(mcoClient)
	e2e.Logf("Initial state: %d nodes, %d machines in worker MCP", len(nodes.Items), mcpCount)
	return initialState{
		nodes:          nodes,
		nodeCount:      len(nodes.Items),
		mcoClient:      mcoClient,
		workerMCPCount: mcpCount,
	}
}

func getWorkerMCPMachineCount(mcoClient machineconfigclient.Interface) int32 {
	mcp, err := mcoClient.MachineconfigurationV1().MachineConfigPools().Get(context.Background(), "worker", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get worker MachineConfigPool")
	return mcp.Status.MachineCount
}

// provisionExtraWorker deploys the extra worker BMH from extraworkers-secret, patches it with
// the fields needed for provisioning, and waits for it to reach the "provisioned" state.
func provisionExtraWorker(oc *exutil.CLI, helper *baremetal.BaremetalTestHelper) {
	// DeployExtraWorker returns (BMH, BMC secret) — we only need the BMH object.
	host, _ := helper.DeployExtraWorker(0)
	e2e.Logf("Extra worker BMH %s deployed and reached available state", host.GetName())

	bmhClient := oc.AdminDynamicClient().Resource(bmhGVR).Namespace(machineAPINamespace)

	triggerBMHProvisioning(bmhClient, host.GetName())
	waitForBMHProvisioned(bmhClient, host.GetName())
}

func approveWorkerCSRs(oc *exutil.CLI) {
	approvedCount := apis.ApproveCSRs(oc, 10*time.Minute, 1*time.Minute, 2)
	o.Expect(approvedCount).To(o.BeNumerically(">=", 2),
		fmt.Sprintf("Expected at least 2 CSRs (kubelet client + serving) but only approved %d", approvedCount))
}

// waitForNewWorkerReady polls until a new worker node (not present in initialNodes) becomes Ready.
func waitForNewWorkerReady(oc *exutil.CLI, initialNodes *corev1.NodeList) string {
	var newWorkerName string

	err := wait.PollUntilContextTimeout(context.Background(), utils.ThirtySecondPollInterval, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: utils.LabelNodeRoleWorker,
		})
		if err != nil {
			e2e.Logf("Failed to list worker nodes: %v", err)
			return false, nil
		}
		for i := range nodes.Items {
			node := &nodes.Items[i]
			if isNodeInList(node.Name, initialNodes.Items) {
				continue
			}
			if utils.IsNodeReady(oc, node.Name) {
				newWorkerName = node.Name
				e2e.Logf("New worker node %s is Ready", node.Name)
				return true, nil
			}
			e2e.Logf("New worker node %s found but not yet Ready", node.Name)
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "Timed out waiting for new worker node to become Ready")
	o.Expect(newWorkerName).NotTo(o.BeEmpty(), "Expected to find a new worker node")
	return newWorkerName
}

// verifyNodeMembership checks that the new node increased the cluster node count and has the worker label.
func verifyNodeMembership(oc *exutil.CLI, newWorkerName string, initialNodeCount int) *corev1.Node {
	finalNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(len(finalNodes.Items)).To(o.Equal(initialNodeCount+1),
		fmt.Sprintf("Expected %d nodes but found %d", initialNodeCount+1, len(finalNodes.Items)))

	newNode, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), newWorkerName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	_, hasWorkerLabel := newNode.Labels[utils.LabelNodeRoleWorker]
	o.Expect(hasWorkerLabel).To(o.BeTrue(),
		fmt.Sprintf("Expected node %s to have label %s", newWorkerName, utils.LabelNodeRoleWorker))

	return newNode
}

func verifyNodeHealth(newNode *corev1.Node) {
	for _, condition := range newNode.Status.Conditions {
		switch condition.Type {
		case corev1.NodeMemoryPressure, corev1.NodeDiskPressure, corev1.NodePIDPressure:
			o.Expect(condition.Status).To(o.Equal(corev1.ConditionFalse),
				fmt.Sprintf("Expected node %s condition %s to be False but got %s",
					newNode.Name, condition.Type, condition.Status))
		}
	}
	e2e.Logf("Node %s has no pressure conditions", newNode.Name)
}

func verifyPodSchedulable(oc *exutil.CLI, nodeName string) {
	testPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "extra-worker-sched-",
			Namespace:    "default",
		},
		Spec: corev1.PodSpec{
			NodeName:      nodeName,
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "test",
					Image:   image.ShellImage(),
					Command: []string{"/bin/bash", "-c", "echo schedulability-ok"},
				},
			},
		},
	}
	createdPod, err := oc.AdminKubeClient().CoreV1().Pods("default").Create(context.Background(), testPod, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create test pod on node %s", nodeName)
	defer func() {
		_ = oc.AdminKubeClient().CoreV1().Pods("default").Delete(context.Background(), createdPod.Name, metav1.DeleteOptions{})
	}()

	err = wait.PollUntilContextTimeout(context.Background(), utils.FiveSecondPollInterval, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := oc.AdminKubeClient().CoreV1().Pods("default").Get(ctx, createdPod.Name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		e2e.Logf("Test pod %s phase: %s", createdPod.Name, pod.Status.Phase)
		return pod.Status.Phase == corev1.PodSucceeded, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "Test pod did not reach Succeeded on node %s", nodeName)

	logs := getPodLogs(oc, "default", createdPod.Name)
	o.Expect(logs).To(o.ContainSubstring("schedulability-ok"),
		fmt.Sprintf("Expected test pod output to contain 'schedulability-ok' but got: %s", logs))
	e2e.Logf("Pod successfully scheduled and ran on node %s", nodeName)
}

func getPodLogs(oc *exutil.CLI, namespace, podName string) string {
	req := oc.AdminKubeClient().CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{})
	logBytes, err := req.DoRaw(context.Background())
	if err != nil {
		e2e.Logf("Failed to get logs for pod %s/%s: %v", namespace, podName, err)
		return ""
	}
	return string(logBytes)
}

func waitForWorkerMCPConvergence(mcoClient machineconfigclient.Interface, initialMachineCount int32) {
	expectedCount := initialMachineCount + 1
	err := wait.PollUntilContextTimeout(context.Background(), utils.ThirtySecondPollInterval, 20*time.Minute, true, func(ctx context.Context) (bool, error) {
		mcp, err := mcoClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, "worker", metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Failed to get worker MCP: %v", err)
			return false, nil
		}
		s := mcp.Status
		e2e.Logf("Worker MCP: machineCount=%d, readyMachineCount=%d, updatedMachineCount=%d, degradedMachineCount=%d",
			s.MachineCount, s.ReadyMachineCount, s.UpdatedMachineCount, s.DegradedMachineCount)
		return s.MachineCount == expectedCount &&
			s.ReadyMachineCount == s.MachineCount &&
			s.UpdatedMachineCount == s.MachineCount &&
			s.DegradedMachineCount == 0, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "Worker MachineConfigPool did not converge after adding new node")
	e2e.Logf("Worker MachineConfigPool converged with %d machines", expectedCount)
}

func verifyKubeletVersion(oc *exutil.CLI, newNode *corev1.Node) {
	masterNodes, err := utils.GetNodes(oc, utils.LabelNodeRoleControlPlane)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(masterNodes.Items).NotTo(o.BeEmpty(), "Expected at least one control-plane node")

	workerVersion := newNode.Status.NodeInfo.KubeletVersion
	masterVersions := make(map[string]bool)
	for i := range masterNodes.Items {
		masterVersions[masterNodes.Items[i].Status.NodeInfo.KubeletVersion] = true
	}
	e2e.Logf("Control-plane kubelet versions: %v, new worker kubelet version: %s", masterVersions, workerVersion)
	o.Expect(masterVersions).To(o.HaveKey(workerVersion),
		fmt.Sprintf("New node %s kubelet version %s does not match any control-plane kubelet version %v",
			newNode.Name, workerVersion, masterVersions))
}

// --- BMH lifecycle helpers ---

// triggerBMHProvisioning patches an available BMH with the fields needed to start provisioning:
// customDeploy method, bootMode, rootDeviceHints, and userData. The extraworkers-secret BMH only
// contains basic connectivity fields (bmc, bootMACAddress) — without customDeploy the BMH stays
// "available" forever, and without userData the node boots CoreOS but never fetches its ignition
// config from MCS, so no kubelet starts and no CSRs are generated.
//
// NOTE: bootMode=UEFI and rootDeviceHints=/dev/sda are specific to dev-scripts VMs (libvirt/QEMU).
// Real hardware deployments would need values derived from the actual hardware inventory.
func triggerBMHProvisioning(bmhClient dynamic.ResourceInterface, bmhName string) {
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"customDeploy": map[string]interface{}{
				"method": "install_coreos",
			},
			"bootMode": "UEFI",
			"rootDeviceHints": map[string]interface{}{
				"deviceName": "/dev/sda",
			},
			"userData": map[string]interface{}{
				"name":      "worker-user-data-managed",
				"namespace": machineAPINamespace,
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	o.Expect(err).NotTo(o.HaveOccurred())

	_, err = bmhClient.Patch(context.Background(), bmhName, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to patch BMH %s with provisioning config", bmhName)

	e2e.Logf("Patched BMH %s with customDeploy=install_coreos, bootMode=UEFI, rootDeviceHints=/dev/sda, userData=worker-user-data-managed", bmhName)
}

// waitForBMHProvisioned polls until the BMH reaches the "provisioned" state.
func waitForBMHProvisioned(bmhClient dynamic.ResourceInterface, bmhName string) {
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, extraWorkerProvisioningTimeout, true, func(ctx context.Context) (bool, error) {
		host, err := bmhClient.Get(ctx, bmhName, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Failed to get BMH %s: %v", bmhName, err)
			return false, nil
		}

		state, found, err := unstructured.NestedString(host.Object, "status", "provisioning", "state")
		if err != nil || !found {
			e2e.Logf("BMH %s provisioning state not found yet", bmhName)
			return false, nil
		}

		e2e.Logf("BMH %s provisioning state: %s", bmhName, state)

		if state == "provisioned" {
			return true, nil
		}
		if state == "error" {
			errMsg, _, _ := unstructured.NestedString(host.Object, "status", "errorMessage")
			return false, fmt.Errorf("BMH %s entered error state: %s", bmhName, errMsg)
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "Timed out waiting for BMH %s to reach provisioned state", bmhName)
}

// --- Cleanup helpers ---

// deleteExtraWorkerBMHs deletes all BMHs whose name contains "extraworker" and waits up to
// 30 minutes for each to disappear. The long timeout is necessary because Metal3 deprovisioning
// a provisioned BMH (power-off + disk clean via Redfish) routinely takes >5 minutes.
//
// This intentionally does NOT touch arbiter/master BMHs: a name-prefix filter avoids the
// mistake of targeting all provisioned BMHs in the namespace.
func deleteExtraWorkerBMHs(bmhClient dynamic.ResourceInterface) {
	bmhList, err := bmhClient.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		e2e.Logf("Failed to list BMHs for cleanup: %v", err)
		return
	}

	for i := range bmhList.Items {
		name := bmhList.Items[i].GetName()
		if !strings.Contains(name, "extraworker") {
			continue
		}

		e2e.Logf("Deleting extraworker BMH %s", name)
		if err := bmhClient.Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
			if kerrors.IsNotFound(err) {
				e2e.Logf("BMH %s already deleted", name)
				continue
			}
			e2e.Logf("Failed to delete BMH %s: %v", name, err)
			continue
		}

		err := wait.PollUntilContextTimeout(context.Background(), 15*time.Second, 30*time.Minute, true, func(ctx context.Context) (bool, error) {
			_, err := bmhClient.Get(ctx, name, metav1.GetOptions{})
			if kerrors.IsNotFound(err) {
				e2e.Logf("BMH %s deleted", name)
				return true, nil
			}
			e2e.Logf("Waiting for BMH %s to be deleted...", name)
			return false, nil
		})
		if err != nil {
			e2e.Logf("BMH %s was not deleted within timeout: %v", name, err)
		}
	}
}

// deleteExtraWorkerNodes removes any Node objects whose name contains "extraworker" from the
// cluster. After BMH deletion, Metal3 powers off the host but the Node object persists in
// NotReady state. Cleaning it up prevents stale nodes from accumulating on long-lived clusters.
func deleteExtraWorkerNodes(oc *exutil.CLI) {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		e2e.Logf("Failed to list nodes for cleanup: %v", err)
		return
	}
	for i := range nodes.Items {
		name := nodes.Items[i].Name
		if !strings.Contains(name, "extraworker") {
			continue
		}
		e2e.Logf("Deleting stale extra worker node %s", name)
		if err := oc.AdminKubeClient().CoreV1().Nodes().Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
			e2e.Logf("Failed to delete node %s: %v", name, err)
		}
	}
}

// isNodeInList checks whether a node name exists in the given node list.
func isNodeInList(name string, nodes []corev1.Node) bool {
	for i := range nodes {
		if nodes[i].Name == name {
			return true
		}
	}
	return false
}
