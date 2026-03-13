package two_node

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	"github.com/openshift/origin/test/extended/two_node/utils"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	nodeutil "k8s.io/kubernetes/pkg/util/node"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	etcdResourceRecoveryTimeout = 5 * time.Minute  // Time for etcd-clone to restart and stabilize
	longRecoveryTimeout = 10 * time.Minute // Time for container kill or standby/unstandby recovery

	crmAttributeName  = "learner_node" // The CRM attribute under test
	pcsWaitTimeout    = 120            // Seconds for pcs --wait flag
	etcdCloneResource = "etcd-clone"   // Pacemaker clone resource name

	// activeCountLogPattern is the pacemaker log message emitted when get_truly_active_resources_count()
	// is called during the start action.
	activeCountLogPattern = "active etcd resources"
	// unexpectedCountError is the error message that should NOT appear after a disable/enable cycle.
	unexpectedCountError = "Unexpected active resource count"

	// stoppingResourcesLogPattern is the pacemaker log message emitted by leave_etcd_member_list()
	// when it counts how many etcd resources are stopping concurrently.
	stoppingResourcesLogPattern = "stopping etcd resources"
	// delayStopLogPattern is the pacemaker log message emitted when the alphabetically second
	// node delays its stop to prevent simultaneous etcd member removal and WAL corruption.
	delayStopLogPattern = "delaying stop for"

)

// learnerCleanupResult holds the parsed output from the disable/enable cycle script.
type learnerCleanupResult struct {
	// StopQueryRC is the return code of crm_attribute --query after the stop operation.
	// RC=6 with "No such device or address" means the attribute was successfully cleared.
	StopQueryRC     string
	StopQueryResult string
	// StartQueryRC is the return code of crm_attribute --query after the start operation.
	StartQueryRC     string
	StartQueryResult string
	// RawOutput is the full script output for diagnostics.
	RawOutput string
}

// isAttributeCleared returns true if the crm_attribute query indicates the attribute was deleted.
// When the attribute doesn't exist, crm_attribute returns RC=6 and prints "No such device or address".
func isAttributeCleared(rc, result string) bool {
	return rc == "6" || strings.Contains(result, "No such device or address")
}

// pcsDisableScript returns a bash snippet that disables a resource and exits on failure.
// On failure it re-enables the resource as a safety net before exiting.
func pcsDisableScript(resource string, timeout int) string {
	return fmt.Sprintf(`sudo pcs resource disable %[1]s --wait=%[2]d
		DISABLE_RC=$?
		if [ $DISABLE_RC -ne 0 ]; then
			echo "DISABLE_FAILED"
			sudo pcs resource enable %[1]s --wait=%[2]d 2>/dev/null || true
			exit 1
		fi`, resource, timeout)
}

// pcsEnableScript returns a bash snippet that enables a resource and exits on failure.
func pcsEnableScript(resource string, timeout int) string {
	return fmt.Sprintf(`sudo pcs resource enable %s --wait=%d
		ENABLE_RC=$?
		if [ $ENABLE_RC -ne 0 ]; then
			echo "ENABLE_FAILED"
			exit 1
		fi`, resource, timeout)
}

// queryCRMAttributeScript returns a bash snippet that queries an attribute and echoes
// the result with the given label prefix (e.g. "STOP" → "STOP_RC=...", "STOP_RESULT=...").
func queryCRMAttributeScript(attr, label string) string {
	return fmt.Sprintf(`%[1]s_RESULT=$(sudo crm_attribute --query --name %[2]s 2>&1); %[1]s_RC=$?
		echo "%[1]s_RC=${%[1]s_RC}"
		echo "%[1]s_RESULT=${%[1]s_RESULT}"`, label, attr)
}

// injectCRMAttributeScript returns a bash snippet that sets a CRM attribute to the given value.
func injectCRMAttributeScript(attr, value string) string {
	return fmt.Sprintf(`sudo crm_attribute --name %s --update %s`, attr, value)
}

// runDisableEnableCycle executes the full disable/enable cycle as a single compound command.
//
// This must run as one bash invocation because disabling etcd-clone stops etcd, which brings
// down the API server — no new debug containers can be created until etcd is re-enabled.
// The debug pod is created while the API is still up; the bash process then runs locally on
// the node and does not need the API for subsequent commands.
//
// The initial inject is also included in the compound command because the resource agent's
// monitor action calls reconcile_member_state() which clears learner_node every few seconds.
// A separate inject would be race-conditioned by the monitor.
//
// The script performs:
//  1. Inject stale learner_node attribute
//  2. Disable etcd-clone (waits for stop to complete)
//  3. Query learner_node attribute (should be cleared by the resource agent's stop action)
//  4. Re-inject the stale learner_node attribute
//  5. Enable etcd-clone (waits for start to complete)
//  6. Query learner_node attribute (should be cleared by the resource agent's start action)
func runDisableEnableCycle(oc *exutil.CLI, nodeName string) (learnerCleanupResult, error) {
	script := strings.Join([]string{
		injectCRMAttributeScript(crmAttributeName, nodeName),
		pcsDisableScript(etcdCloneResource, pcsWaitTimeout),
		queryCRMAttributeScript(crmAttributeName, "STOP"),
		injectCRMAttributeScript(crmAttributeName, nodeName),
		pcsEnableScript(etcdCloneResource, pcsWaitTimeout),
		queryCRMAttributeScript(crmAttributeName, "START"),
	}, "\n")

	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
		oc, nodeName, "default", "bash", "-c", script)
	framework.Logf("Disable/enable cycle output:\n%s", output)

	// err may be non-nil if the debug container cleanup fails while etcd is down.
	// The actual test results are captured in stdout.
	if err != nil {
		framework.Logf("Disable/enable cycle returned error (may be expected due to API disruption): %v", err)
	}

	return learnerCleanupResult{
		StopQueryRC:      extractValue(output, "STOP_RC="),
		StopQueryResult:  extractValue(output, "STOP_RESULT="),
		StartQueryRC:     extractValue(output, "START_RC="),
		StartQueryResult: extractValue(output, "START_RESULT="),
		RawOutput:        output,
	}, err
}

// extractValue finds a line starting with the given prefix and returns the value after it.
func extractValue(output, prefix string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return ""
}

// waitForAllNodesReady checks that the expected number of nodes exist and all are Ready.
func waitForAllNodesReady(oc *exutil.CLI, expectedCount int) error {
	nodeList, err := utils.GetNodes(oc, utils.AllNodes)
	if err != nil {
		return fmt.Errorf("failed to retrieve nodes: %v", err)
	}
	if len(nodeList.Items) != expectedCount {
		return fmt.Errorf("expected %d nodes, found %d", expectedCount, len(nodeList.Items))
	}
	for _, node := range nodeList.Items {
		nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(
			context.Background(), node.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get node %s: %v", node.Name, err)
		}
		if !nodeutil.IsNodeReady(nodeObj) {
			return fmt.Errorf("node %s is not Ready", node.Name)
		}
	}
	return nil
}

// verifyEtcdCloneStartedOnAllNodes checks that pcs status shows etcd-clone Started on all given nodes.
// Clone resources use the format "Started: [ node1 node2 ]", so we extract the etcd-clone section
// and look for each node name on a "Started" line within that section.
func verifyEtcdCloneStartedOnAllNodes(oc *exutil.CLI, execNodeName string, nodes []corev1.Node) error {
	statusOutput, err := exutil.DebugNodeRetryWithOptionsAndChroot(
		oc, execNodeName, "default", "bash", "-c", "sudo pcs status")
	if err != nil {
		return fmt.Errorf("failed to get pcs status: %v", err)
	}
	etcdIdx := strings.Index(statusOutput, "etcd-clone")
	if etcdIdx == -1 {
		return fmt.Errorf("etcd-clone not found in pcs status:\n%s", statusOutput)
	}
	etcdSection := statusOutput[etcdIdx:]
	for _, node := range nodes {
		found := false
		for _, line := range strings.Split(etcdSection, "\n") {
			if strings.Contains(line, "Started") && strings.Contains(line, node.Name) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("etcd-clone not Started on %s, status:\n%s", node.Name, statusOutput)
		}
	}
	framework.Logf("Final pcs status:\n%s", statusOutput)
	return nil
}

// getPacemakerLogGrep runs a grep against /var/log/pacemaker/pacemaker.log on the given node
// and returns the matching lines. Returns empty string if no matches found.
func getPacemakerLogGrep(oc *exutil.CLI, nodeName, pattern string) (string, error) {
	cmd := fmt.Sprintf(`grep "%s" /var/log/pacemaker/pacemaker.log | tail -5`, pattern)
	return exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "default", "bash", "-c", cmd)
}

// extractFailedActionsSection extracts everything after "Failed Resource Actions:" from pcs status output.
// In pacemaker, this section lists historical failures that haven't been cleared with `pcs resource cleanup`.
func extractFailedActionsSection(pcsOutput string) string {
	for _, marker := range []string{"Failed Resource Actions:", "Failed Resource Actions"} {
		idx := strings.Index(pcsOutput, marker)
		if idx != -1 {
			return pcsOutput[idx:]
		}
	}
	return ""
}

// runSimpleDisableEnableCycle disables and re-enables etcd-clone as a single compound command.
// Returns the combined output. The error may be non-nil due to API disruption while etcd is down.
func runSimpleDisableEnableCycle(oc *exutil.CLI, nodeName string) string {
	script := strings.Join([]string{
		pcsDisableScript(etcdCloneResource, pcsWaitTimeout),
		pcsEnableScript(etcdCloneResource, pcsWaitTimeout),
	}, "\n")

	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
		oc, nodeName, "default", "bash", "-c", script)
	framework.Logf("Disable/enable cycle output:\n%s", output)

	if err != nil {
		framework.Logf("Disable/enable cycle returned error (may be expected due to API disruption): %v", err)
	}

	o.Expect(output).NotTo(o.ContainSubstring("DISABLE_FAILED"),
		"pcs resource disable should succeed")
	o.Expect(output).NotTo(o.ContainSubstring("ENABLE_FAILED"),
		"pcs resource enable should succeed")

	return output
}

// expectPacemakerLogFound verifies that at least one node's pacemaker log contains the given pattern.
func expectPacemakerLogFound(oc *exutil.CLI, nodes []corev1.Node, pattern, description string) {
	var found bool
	for _, node := range nodes {
		logOutput, logErr := getPacemakerLogGrep(oc, node.Name, pattern)
		if logErr != nil {
			framework.Logf("Warning: failed to grep pacemaker log on %s: %v", node.Name, logErr)
			continue
		}
		if strings.TrimSpace(logOutput) != "" {
			framework.Logf("%s on %s:\n%s", description, node.Name, logOutput)
			found = true
		}
	}
	o.Expect(found).To(o.BeTrue(),
		fmt.Sprintf("Expected at least one node's pacemaker log to contain %s", description))
}

// verifyFinalClusterHealth runs the common end-of-test health checks: etcd cluster status,
// etcd-clone started on both nodes, all nodes ready, and essential operators available.
func verifyFinalClusterHealth(oc *exutil.CLI, execNodeName string, nodes []corev1.Node,
	etcdClientFactory *helpers.EtcdClientFactoryImpl, label string, timeout time.Duration) {

	g.By("Verifying etcd cluster health")
	o.Eventually(func() error {
		return utils.LogEtcdClusterStatus(oc, label, etcdClientFactory)
	}, timeout, utils.FiveSecondPollInterval).ShouldNot(
		o.HaveOccurred(), "etcd cluster should be healthy")

	g.By("Verifying pcs status shows etcd-clone Started on both nodes")
	o.Eventually(func() error {
		return verifyEtcdCloneStartedOnAllNodes(oc, execNodeName, nodes)
	}, timeout, utils.FiveSecondPollInterval).ShouldNot(
		o.HaveOccurred(), "etcd-clone should be Started on both nodes")

	g.By("Verifying both nodes are Ready")
	o.Eventually(func() error {
		return waitForAllNodesReady(oc, 2)
	}, timeout, utils.FiveSecondPollInterval).Should(
		o.Succeed(), "Both nodes should be Ready")

	g.By("Verifying essential operators are available")
	o.Eventually(func() error {
		return utils.ValidateEssentialOperatorsAvailable(oc)
	}, timeout, utils.FiveSecondPollInterval).ShouldNot(
		o.HaveOccurred(), "Essential operators should be available")
}

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/tnf-resilience][Serial][Disruptive] Two Node with Fencing etcd resilience", func() {
	defer g.GinkgoRecover()

	var (
		oc                = exutil.NewCLIWithoutNamespace("tnf-resilience").AsAdmin()
		etcdClientFactory *helpers.EtcdClientFactoryImpl
		setupCompleted    bool
	)

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)

		etcdClientFactory = helpers.NewEtcdClientFactory(oc.KubeClient())

		utils.SkipIfClusterIsNotHealthy(oc, etcdClientFactory)
		setupCompleted = true
	})

	g.AfterEach(func() {
		if !setupCompleted {
			framework.Logf("Test was skipped before setup completed, skipping AfterEach cleanup")
			return
		}

		nodeList, _ := utils.GetNodes(oc, utils.AllNodes)
		if len(nodeList.Items) == 0 {
			framework.Logf("Warning: Could not retrieve nodes during cleanup")
			return
		}
		cleanupNode := nodeList.Items[0]

		g.By("Cleanup: Ensuring all nodes are unstandby")
		for _, node := range nodeList.Items {
			if _, err := exutil.DebugNodeRetryWithOptionsAndChroot(
				oc, cleanupNode.Name, "default", "bash", "-c",
				fmt.Sprintf("sudo pcs node unstandby %s 2>/dev/null; true", node.Name)); err != nil {
				framework.Logf("Warning: Failed to unstandby %s: %v", node.Name, err)
			}
		}

		g.By("Cleanup: Ensuring etcd-clone is enabled")
		if err := utils.EnablePacemakerResource(oc, cleanupNode.Name, etcdCloneResource); err != nil {
			framework.Logf("Warning: Failed to enable etcd-clone during cleanup: %v", err)
		}

		g.By("Cleanup: Clearing any stale learner_node CRM attribute")
		utils.DeleteCRMAttribute(oc, cleanupNode.Name, crmAttributeName)

		g.By("Cleanup: Running pcs resource cleanup to clear failed actions")
		if output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
			oc, cleanupNode.Name, "default", "bash", "-c", "sudo pcs resource cleanup"); err != nil {
			framework.Logf("Warning: Failed to run pcs resource cleanup during AfterEach: %v", err)
		} else {
			framework.Logf("PCS resource cleanup output: %s", output)
		}

		g.By("Cleanup: Waiting for both nodes to become Ready")
		o.Eventually(func() error {
			return waitForAllNodesReady(oc, 2)
		}, longRecoveryTimeout, utils.FiveSecondPollInterval).Should(
			o.Succeed(), "Both nodes must be Ready after cleanup")

		g.By("Cleanup: Validating etcd cluster health")
		o.Eventually(func() error {
			return utils.LogEtcdClusterStatus(oc, "AfterEach cleanup", etcdClientFactory)
		}, longRecoveryTimeout, utils.FiveSecondPollInterval).Should(
			o.Succeed(), "Etcd cluster must be healthy after cleanup")
	})

	// This test verifies that the resource agent's stop and start actions both clear
	// a stale learner_node CRM attribute. A stale attribute would prevent a node from
	// completing its etcd rejoin because the start action polls this attribute.
	g.It("should clean up stale learner_node attribute during etcd-clone stop and start operations", func() {
		nodeList, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodeList.Items)).To(o.Equal(2), "Expected exactly 2 nodes for two-node cluster")

		nodes := nodeList.Items
		execNode := nodes[0]

		g.By("Verifying both nodes are healthy before test")
		o.Eventually(func() error {
			return waitForAllNodesReady(oc, 2)
		}, nodeIsHealthyTimeout, utils.FiveSecondPollInterval).Should(
			o.Succeed(), "Both nodes should be Ready before test")

		// Run inject + disable/enable cycle as a single compound command.
		// The inject must be part of the compound command because the resource agent's
		// monitor action calls reconcile_member_state() which clears learner_node
		// every few seconds — a separate inject would be race-conditioned.
		g.By("Running inject + disable/enable cycle to verify learner_node cleanup on stop and start")
		result, _ := runDisableEnableCycle(oc, execNode.Name)

		// Verify the disable/enable completed successfully
		o.Expect(result.RawOutput).NotTo(o.ContainSubstring("DISABLE_FAILED"),
			"pcs resource disable should succeed")
		o.Expect(result.RawOutput).NotTo(o.ContainSubstring("ENABLE_FAILED"),
			"pcs resource enable should succeed")

		// Verify: attribute was cleared by the resource agent's stop action
		g.By("Verifying learner_node attribute was cleared after etcd-clone stop")
		o.Expect(result.StopQueryRC).NotTo(o.BeEmpty(),
			fmt.Sprintf("Expected STOP_RC in script output, raw output:\n%s", result.RawOutput))
		o.Expect(isAttributeCleared(result.StopQueryRC, result.StopQueryResult)).To(o.BeTrue(),
			fmt.Sprintf("Expected learner_node to be cleared after stop (RC=%s, result=%s)",
				result.StopQueryRC, result.StopQueryResult))
		framework.Logf("STOP path verified: learner_node was cleared by the resource agent stop action")

		g.By("Verifying learner_node attribute was cleared after etcd-clone start")
		o.Expect(result.StartQueryRC).NotTo(o.BeEmpty(),
			fmt.Sprintf("Expected START_RC in script output, raw output:\n%s", result.RawOutput))
		o.Expect(isAttributeCleared(result.StartQueryRC, result.StartQueryResult)).To(o.BeTrue(),
			fmt.Sprintf("Expected learner_node to be cleared after start (RC=%s, result=%s)",
				result.StartQueryRC, result.StartQueryResult))
		framework.Logf("START path verified: learner_node was cleared by the resource agent start action")

		verifyFinalClusterHealth(oc, execNode.Name, nodes, etcdClientFactory,
			"after learner cleanup test", etcdResourceRecoveryTimeout)
	})

	// This test verifies that get_truly_active_resources_count() in the podman-etcd resource agent
	// correctly differentiates truly active resources from those being stopped.
	//
	// A disable/enable cycle triggers this code path because both instances restart cleanly
	// without force-new-cluster being pre-set, entering the branch that calls the function
	// and logs the active resource count.
	g.It("should exclude stopping resources from active count during etcd-clone disable/enable cycle", func() {
		nodeList, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodeList.Items)).To(o.Equal(2), "Expected exactly 2 nodes for two-node cluster")

		nodes := nodeList.Items
		execNode := nodes[0]

		g.By("Verifying both nodes are healthy before test")
		o.Eventually(func() error {
			return waitForAllNodesReady(oc, 2)
		}, nodeIsHealthyTimeout, utils.FiveSecondPollInterval).Should(
			o.Succeed(), "Both nodes should be Ready before test")

		g.By("Running etcd-clone disable/enable cycle to trigger active resource count logic")
		runSimpleDisableEnableCycle(oc, execNode.Name)

		g.By("Waiting for etcd cluster to recover after disable/enable cycle")
		o.Eventually(func() error {
			return utils.LogEtcdClusterStatus(oc, "after disable/enable cycle", etcdClientFactory)
		}, etcdResourceRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(
			o.HaveOccurred(), "etcd cluster should recover after disable/enable cycle")

		g.By("Verifying pcs status shows etcd-clone Started on both nodes")
		o.Eventually(func() error {
			return verifyEtcdCloneStartedOnAllNodes(oc, execNode.Name, nodes)
		}, etcdResourceRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(
			o.HaveOccurred(), "etcd-clone should be Started on both nodes after recovery")

		g.By("Checking pacemaker logs for correct active resource count logic")
		expectPacemakerLogFound(oc, nodes, activeCountLogPattern, "Active count log entries")

		g.By("Verifying no 'Unexpected active resource count' errors in pacemaker logs")
		for _, node := range nodes {
			errorOutput, logErr := getPacemakerLogGrep(oc, node.Name, unexpectedCountError)
			if logErr != nil {
				framework.Logf("Warning: failed to grep pacemaker log on %s: %v", node.Name, logErr)
				continue
			}
			o.Expect(strings.TrimSpace(errorOutput)).To(o.BeEmpty(),
				fmt.Sprintf("Expected no 'Unexpected active resource count' errors on %s", node.Name))
		}

		verifyFinalClusterHealth(oc, execNode.Name, nodes, etcdClientFactory,
			"after active count test", etcdResourceRecoveryTimeout)
	})

	// This test verifies that podman-etcd prevents simultaneous etcd member removal
	// when both nodes receive a graceful shutdown request.
	//
	// When etcd-clone is disabled, both nodes stop concurrently. The leave_etcd_member_list()
	// function detects this by counting the stopping resources. The alphabetically second node
	// is delayed by DELAY_SECOND_NODE_LEAVE_SEC (10s) to prevent WAL corruption.
	g.It("should delay the second node stop to prevent simultaneous etcd member removal", func() {
		nodeList, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodeList.Items)).To(o.Equal(2), "Expected exactly 2 nodes for two-node cluster")

		nodes := nodeList.Items
		execNode := nodes[0]

		g.By("Verifying both nodes are healthy before test")
		o.Eventually(func() error {
			return waitForAllNodesReady(oc, 2)
		}, nodeIsHealthyTimeout, utils.FiveSecondPollInterval).Should(
			o.Succeed(), "Both nodes should be Ready before test")

		g.By("Running etcd-clone disable/enable cycle to trigger simultaneous stop logic")
		runSimpleDisableEnableCycle(oc, execNode.Name)

		g.By("Waiting for etcd cluster to recover after disable/enable cycle")
		o.Eventually(func() error {
			return utils.LogEtcdClusterStatus(oc, "after disable/enable cycle", etcdClientFactory)
		}, etcdResourceRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(
			o.HaveOccurred(), "etcd cluster should recover after disable/enable cycle")

		g.By("Verifying pcs status shows etcd-clone Started on both nodes")
		o.Eventually(func() error {
			return verifyEtcdCloneStartedOnAllNodes(oc, execNode.Name, nodes)
		}, etcdResourceRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(
			o.HaveOccurred(), "etcd-clone should be Started on both nodes after recovery")

		g.By("Checking pacemaker logs for stopping resource count detection")
		expectPacemakerLogFound(oc, nodes, stoppingResourcesLogPattern, "Stopping resources log entries")

		g.By("Verifying delay intervention was applied to prevent simultaneous member removal")
		expectPacemakerLogFound(oc, nodes, delayStopLogPattern, "Delay intervention log")

		verifyFinalClusterHealth(oc, execNode.Name, nodes, etcdClientFactory,
			"after simultaneous stop test", etcdResourceRecoveryTimeout)
	})

	// This test verifies that an abrupt termination of the etcd container triggers a
	// coordinated "Error occurred" monitor state on both nodes before the cluster
	// self-heals automatically.
	//
	// When a local etcd container is killed, the podman-etcd resource agent must
	// coordinate recovery with the peer node. The surviving node sets force_new_cluster
	// and the killed node's etcd restarts and joins as a learner. During this process,
	// both nodes briefly enter a coordinated failed state visible in pcs status as
	// "Failed Resource Actions".
	g.It("should coordinate recovery with peer when local etcd container is killed", func() {
		nodeList, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodeList.Items)).To(o.Equal(2), "Expected exactly 2 nodes for two-node cluster")

		nodes := nodeList.Items
		targetNode := nodes[1] // Kill etcd on the second node
		execNode := nodes[0]   // Use first node for pcs status checks after recovery

		g.By("Verifying both nodes are healthy before test")
		o.Eventually(func() error {
			return waitForAllNodesReady(oc, 2)
		}, nodeIsHealthyTimeout, utils.FiveSecondPollInterval).Should(
			o.Succeed(), "Both nodes should be Ready before test")

		// Kill etcd container on the target node.
		g.By(fmt.Sprintf("Killing etcd container on %s", targetNode.Name))
		_, err = exutil.DebugNodeRetryWithOptionsAndChroot(
			oc, targetNode.Name, "openshift-etcd",
			"bash", "-c", "podman kill etcd 2>/dev/null")
		o.Expect(err).To(o.BeNil(), "Expected to kill etcd container without command errors")

		// Wait for the cluster to self-heal.
		g.By("Waiting for etcd cluster to self-heal after container kill")
		o.Eventually(func() error {
			return utils.LogEtcdClusterStatus(oc, "after container kill", etcdClientFactory)
		}, longRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(
			o.HaveOccurred(), "etcd cluster should self-heal after container kill")

		g.By("Verifying pcs status shows etcd-clone Started on both nodes")
		o.Eventually(func() error {
			return verifyEtcdCloneStartedOnAllNodes(oc, execNode.Name, nodes)
		}, longRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(
			o.HaveOccurred(), "etcd-clone should be Started on both nodes after recovery")

		// Verify that the coordinated failure was observed.
		g.By("Checking pcs status for coordinated 'Failed Resource Actions' on both nodes")
		pcsOutput, statusErr := exutil.DebugNodeRetryWithOptionsAndChroot(
			oc, execNode.Name, "default", "bash", "-c", "sudo pcs status")
		o.Expect(statusErr).ShouldNot(o.HaveOccurred(), "Expected to get pcs status without error")
		framework.Logf("PCS status after recovery:\n%s", pcsOutput)

		failedSection := extractFailedActionsSection(pcsOutput)
		o.Expect(failedSection).NotTo(o.BeEmpty(),
			"Expected pcs status to contain 'Failed Resource Actions' section after container kill")
		framework.Logf("Failed Resource Actions section:\n%s", failedSection)

		o.Expect(failedSection).To(o.ContainSubstring("etcd"),
			"Expected Failed Resource Actions to reference etcd")

		for _, node := range nodes {
			if strings.Contains(failedSection, node.Name) {
				framework.Logf("Coordinated failure confirmed: node %s found in Failed Resource Actions", node.Name)
			} else {
				framework.Logf("Node %s NOT found in Failed Resource Actions", node.Name)
			}
		}
		for _, node := range nodes {
			o.Expect(failedSection).To(o.ContainSubstring(node.Name),
				fmt.Sprintf("Expected Failed Resource Actions to show failure on %s for coordinated recovery", node.Name))
		}

		verifyFinalClusterHealth(oc, execNode.Name, nodes, etcdClientFactory,
			"after coordinated recovery test", longRecoveryTimeout)
	})

	// This test verifies that the podman-etcd resource agent retries setting
	// CRM attributes when they fail during the force-new-cluster recovery path.
	//
	// When the learner_node CIB attribute is deleted while a node is in standby,
	// the returning node's start action polls for the attribute but finds it missing.
	// Without the retry fix, the node gets stuck in the LEARNER=true stage because
	// nobody re-sets the attribute. With the fix, the leader node's monitor detects
	// that a learner member exists in etcd but the learner_node attribute is missing,
	// and retries setting it, allowing the returning node to proceed.
	//
	// Test flow:
	// 1. Put a node in standby (triggers force-new-cluster on the peer)
	// 2. Wait for the standby node to appear as a learner in etcd member list
	// 3. Delete the learner_node CRM attribute
	// 4. Unstandby the node
	// 5. Verify both nodes recover to voting etcd members
	g.It("should retry setting learner_node attribute after deletion during force-new-cluster recovery", func() {
		nodeList, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodeList.Items)).To(o.Equal(2), "Expected exactly 2 nodes for two-node cluster")

		nodes := nodeList.Items
		execNode := nodes[0]    // Stays active, runs solo during standby
		standbyNode := nodes[1] // Will be put in standby

		g.By("Verifying both nodes are healthy before test")
		o.Eventually(func() error {
			return waitForAllNodesReady(oc, 2)
		}, nodeIsHealthyTimeout, utils.FiveSecondPollInterval).Should(
			o.Succeed(), "Both nodes should be Ready before test")

		// Put the standby node in standby mode.
		g.By(fmt.Sprintf("Putting %s in standby", standbyNode.Name))
		output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
			oc, execNode.Name, "default", "bash", "-c",
			fmt.Sprintf("sudo pcs node standby %s", standbyNode.Name))
		o.Expect(err).ShouldNot(o.HaveOccurred(),
			fmt.Sprintf("Expected pcs node standby to succeed, output: %s", output))
		framework.Logf("PCS node standby output: %s", output)

		// Wait for force-new-cluster recovery to complete.
		g.By(fmt.Sprintf("Waiting for %s to appear as learner in etcd member list", standbyNode.Name))
		o.Eventually(func() error {
			members, err := utils.GetMembers(etcdClientFactory)
			if err != nil {
				return fmt.Errorf("failed to get etcd members: %v", err)
			}
			_, isLearner, err := utils.GetMemberState(&standbyNode, members)
			if err != nil {
				return fmt.Errorf("standby node not in member list yet: %v", err)
			}
			if !isLearner {
				return fmt.Errorf("standby node %s is not a learner yet", standbyNode.Name)
			}
			framework.Logf("Standby node %s confirmed as learner in etcd member list", standbyNode.Name)
			return nil
		}, longRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(
			o.HaveOccurred(), "Standby node should appear as learner in etcd member list")

		g.By("Logging pcs status after standby")
		if pcsOutput, pcsErr := exutil.DebugNodeRetryWithOptionsAndChroot(
			oc, execNode.Name, "default", "bash", "-c", "sudo pcs status"); pcsErr == nil {
			framework.Logf("PCS status after standby:\n%s", pcsOutput)
		}

		// Verify learner_node attribute is set before we delete it
		g.By("Verifying learner_node CRM attribute is set")
		attrOutput, err := utils.QueryCRMAttribute(oc, execNode.Name, crmAttributeName)
		o.Expect(err).ShouldNot(o.HaveOccurred(),
			"Expected learner_node attribute to exist after force-new-cluster recovery")
		framework.Logf("learner_node attribute value: %s", attrOutput)

		// Delete the learner_node attribute to simulate attribute update failure.
		g.By("Deleting learner_node CRM attribute to simulate attribute update failure")
		utils.DeleteCRMAttribute(oc, execNode.Name, crmAttributeName)
		framework.Logf("learner_node attribute deleted")

		// Unstandby the node. With the retry fix, the leader node's monitor detects
		// the missing attribute and re-sets it, allowing the returning node to proceed.
		g.By(fmt.Sprintf("Unstandby %s to trigger etcd rejoin", standbyNode.Name))
		output, err = exutil.DebugNodeRetryWithOptionsAndChroot(
			oc, execNode.Name, "default", "bash", "-c",
			fmt.Sprintf("sudo pcs node unstandby %s", standbyNode.Name))
		o.Expect(err).ShouldNot(o.HaveOccurred(),
			fmt.Sprintf("Expected pcs node unstandby to succeed, output: %s", output))
		framework.Logf("PCS node unstandby output: %s", output)

		// Wait for both nodes to become voting etcd members.
		g.By("Waiting for both nodes to become voting etcd members")
		o.Eventually(func() error {
			members, err := utils.GetMembers(etcdClientFactory)
			if err != nil {
				return fmt.Errorf("failed to get etcd members: %v", err)
			}
			if len(members) != 2 {
				return fmt.Errorf("expected 2 members, found %d", len(members))
			}
			for i := range nodes {
				isStarted, isLearner, err := utils.GetMemberState(&nodes[i], members)
				if err != nil {
					return fmt.Errorf("member %s not found: %v", nodes[i].Name, err)
				}
				if !isStarted {
					return fmt.Errorf("member %s is not started", nodes[i].Name)
				}
				if isLearner {
					return fmt.Errorf("member %s is still a learner", nodes[i].Name)
				}
			}
			framework.Logf("Both etcd members are now voting members")
			return nil
		}, longRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(
			o.HaveOccurred(), "Both nodes should become voting etcd members")

		verifyFinalClusterHealth(oc, execNode.Name, nodes, etcdClientFactory,
			"after attribute retry test", longRecoveryTimeout)
	})
})
