package two_node

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/apis"
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	"github.com/openshift/origin/test/extended/two_node/utils/services"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	nodeutil "k8s.io/kubernetes/pkg/util/node"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	nodeIsHealthyTimeout            = time.Minute
	etcdOperatorIsHealthyTimeout    = time.Minute
	memberHasLeftTimeout            = 5 * time.Minute
	memberIsLeaderTimeout           = 20 * time.Minute
	memberRejoinedLearnerTimeout    = 20 * time.Minute
	memberPromotedVotingTimeout     = 15 * time.Minute
	networkDisruptionDuration       = 15 * time.Second
	vmRestartTimeout                = 5 * time.Minute
	vmUngracefulShutdownTimeout     = 30 * time.Second // Ungraceful VM shutdown is typically fast
	vmGracefulShutdownTimeout       = 10 * time.Minute // Graceful VM shutdown is typically slow
	membersHealthyAfterDoubleReboot = 30 * time.Minute // Includes full VM reboot and etcd member healthy
	progressLogInterval             = time.Minute      // Target interval for progress logging

	etcdResourceRecoveryTimeout = 5 * time.Minute  // Time for etcd-clone to restart and stabilize
	longRecoveryTimeout         = 10 * time.Minute // Time for container kill or standby/unstandby recovery

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

// computeLogInterval calculates poll attempts between progress logs based on poll interval.
func computeLogInterval(pollInterval time.Duration) int {
	if pollInterval <= 0 {
		return 1
	}
	n := int(progressLogInterval / pollInterval)
	if n < 1 {
		return 1
	}
	return n
}

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
	// Scope parsing to the etcd-clone resource block only, stopping at the next
	// resource header or blank line to avoid matching unrelated "Started" lines.
	etcdLines := strings.Split(statusOutput[etcdIdx:], "\n")
	var etcdSection strings.Builder
	etcdSection.WriteString(etcdLines[0])
	for _, line := range etcdLines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || (strings.HasSuffix(trimmed, ":") && !strings.Contains(line, "Started")) {
			break
		}
		etcdSection.WriteString("\n")
		etcdSection.WriteString(line)
	}
	sectionStr := etcdSection.String()
	for _, node := range nodes {
		found := false
		for _, line := range strings.Split(sectionStr, "\n") {
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
// and returns matching lines. If baselineLineCount is non-empty, only lines after that line
// number are searched (using tail +N). Returns empty string if no matches found.
func getPacemakerLogGrep(oc *exutil.CLI, nodeName, pattern, baselineLineCount string) (string, error) {
	var cmd string
	if baselineLineCount != "" {
		// Use tail to skip lines that existed before the test, then grep for pattern
		cmd = fmt.Sprintf(`tail -n +%s /var/log/pacemaker/pacemaker.log | grep -F -- %q | tail -5`,
			baselineLineCount, pattern)
	} else {
		cmd = fmt.Sprintf(`grep -F -- %q /var/log/pacemaker/pacemaker.log | tail -5`, pattern)
	}
	return exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "default", "bash", "-c", cmd)
}

// getPacemakerLogBaselines captures the current line count of the pacemaker log on each node.
// Returns a map of nodeName -> lineCount string. Used to scope log assertions to only lines
// emitted after the baseline, preventing stale log lines from prior tests causing false positives.
func getPacemakerLogBaselines(oc *exutil.CLI, nodes []corev1.Node) map[string]string {
	baselines := make(map[string]string, len(nodes))
	for _, node := range nodes {
		output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
			oc, node.Name, "default", "bash", "-c", "wc -l < /var/log/pacemaker/pacemaker.log")
		if err != nil {
			framework.Logf("Warning: could not get pacemaker log line count from %s: %v", node.Name, err)
			continue
		}
		baselines[node.Name] = strings.TrimSpace(output)
	}
	return baselines
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
// If baselines is non-nil, only log lines after each node's baseline line count are considered.
func expectPacemakerLogFound(oc *exutil.CLI, nodes []corev1.Node, pattern, description string, baselines map[string]string) {
	var found bool
	for _, node := range nodes {
		logOutput, logErr := getPacemakerLogGrep(oc, node.Name, pattern, baselines[node.Name])
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

type hypervisorExtendedConfig struct {
	HypervisorConfig         core.SSHConfig
	HypervisorKnownHostsPath string
}

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Serial][Disruptive] Two Node with Fencing etcd recovery", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = exutil.NewCLIWithoutNamespace("").AsAdmin()
		etcdClientFactory    *helpers.EtcdClientFactoryImpl
		peerNode, targetNode corev1.Node
	)

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)

		etcdClientFactory = helpers.NewEtcdClientFactory(oc.KubeClient())

		// Health check fetches nodes internally and validates node count
		utils.SkipIfClusterIsNotHealthy(oc, etcdClientFactory)

		// Get nodes for test setup (health check already validated 2 nodes exist)
		nodes, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")

		// Select the first index randomly
		randomIndex := rand.Intn(len(nodes.Items))
		peerNode = nodes.Items[randomIndex]
		// Select the remaining index
		targetNode = nodes.Items[(randomIndex+1)%len(nodes.Items)]
	})

	g.AfterEach(func() {
		nodeList, err := utils.GetNodes(oc, utils.AllNodes)
		if err != nil || len(nodeList.Items) == 0 {
			framework.Logf("Warning: Could not retrieve nodes during cleanup: %v", err)
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

	g.It("should recover from graceful node shutdown with etcd member re-addition", func() {
		// Note: In graceful shutdown, the targetNode is deliberately shut down while
		// the peerNode remains running and becomes the etcd leader.
		survivedNode := peerNode
		g.GinkgoT().Printf("Randomly selected %s (%s) to be shut down and %s (%s) to take the lead\n",
			targetNode.Name, targetNode.Status.Addresses[0].Address, peerNode.Name, peerNode.Status.Addresses[0].Address)
		g.By(fmt.Sprintf("Shutting down %s gracefully in 1 minute", targetNode.Name))
		err := exutil.TriggerNodeRebootGraceful(oc.KubeClient(), targetNode.Name)
		o.Expect(err).To(o.BeNil(), "Expected to gracefully shutdown the node without errors")
		time.Sleep(time.Minute)

		g.By(fmt.Sprintf("Ensuring %s leaves the member list (timeout: %v)", targetNode.Name, memberHasLeftTimeout))
		o.Eventually(func() error {
			return helpers.EnsureMemberRemoved(g.GinkgoT(), etcdClientFactory, targetNode.Name)
		}, memberHasLeftTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred())

		g.By(fmt.Sprintf("Ensuring that %s is a healthy voting member and adds %s back as learner (timeout: %v)", peerNode.Name, targetNode.Name, memberIsLeaderTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, false, true, // targetNode expected started == false, learner == true
			memberIsLeaderTimeout, utils.FiveSecondPollInterval)

		g.By(fmt.Sprintf("Ensuring %s rejoins as learner (timeout: %v)", targetNode.Name, memberRejoinedLearnerTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, true, true, // targetNode expected started == true, learner == true
			memberRejoinedLearnerTimeout, utils.FiveSecondPollInterval)

		g.By(fmt.Sprintf("Ensuring %s node is promoted back as voting member (timeout: %v)", targetNode.Name, memberPromotedVotingTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, true, false, // targetNode expected started == true, learner == false
			memberPromotedVotingTimeout, utils.FiveSecondPollInterval)
	})

	g.It("should recover from ungraceful node shutdown with etcd member re-addition", func() {
		// Note: In ungraceful shutdown, the targetNode is forcibly shut down while
		// the peerNode remains running and becomes the etcd leader.
		survivedNode := peerNode
		g.GinkgoT().Printf("Randomly selected %s (%s) to be shut down and %s (%s) to take the lead\n",
			targetNode.Name, targetNode.Status.Addresses[0].Address, peerNode.Name, peerNode.Status.Addresses[0].Address)
		g.By(fmt.Sprintf("Shutting down %s ungracefully in 1 minute", targetNode.Name))
		err := exutil.TriggerNodeRebootUngraceful(oc.KubeClient(), targetNode.Name)
		o.Expect(err).To(o.BeNil(), "Expected to ungracefully shutdown the node without errors", targetNode.Name, err)
		time.Sleep(1 * time.Minute)

		g.By(fmt.Sprintf("Ensuring that %s added %s back as learner (timeout: %v)", peerNode.Name, targetNode.Name, memberIsLeaderTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, false, true, // targetNode expected started == false, learner == true
			memberIsLeaderTimeout, utils.FiveSecondPollInterval)

		g.By(fmt.Sprintf("Ensuring %s rejoins as learner (timeout: %v)", targetNode.Name, memberRejoinedLearnerTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, true, true, // targetNode expected started == true, learner == true
			memberRejoinedLearnerTimeout, utils.FiveSecondPollInterval)

		g.By(fmt.Sprintf("Ensuring %s node is promoted back as voting member (timeout: %v)", targetNode.Name, memberPromotedVotingTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, true, false, // targetNode expected started == true, learner == false
			memberPromotedVotingTimeout, utils.FiveSecondPollInterval)
	})

	g.FIt("should recover from network disruption with etcd member re-addition", func() {
		// Note: In network disruption, the targetNode runs the disruption command that
		// isolates the nodes from each other, creating a split-brain where pacemaker
		// determines which node gets fenced and which becomes the etcd leader.
		g.GinkgoT().Printf("Randomly selected %s (%s) to run the network disruption command\n", targetNode.Name, targetNode.Status.Addresses[0].Address)
		g.By(fmt.Sprintf("Blocking network communication between %s and %s for %v ", targetNode.Name, peerNode.Name, networkDisruptionDuration))
		command, err := exutil.TriggerNetworkDisruption(oc.KubeClient(), &targetNode, &peerNode, networkDisruptionDuration)
		o.Expect(err).To(o.BeNil(), "Expected to disrupt network without errors")
		g.GinkgoT().Printf("command: '%s'\n", command)

		g.By(fmt.Sprintf("Ensuring cluster recovery with proper leader/learner roles after network disruption (timeout: %v)", memberIsLeaderTimeout))
		// Note: The fenced node may recover quickly and already be started when we get
		// the first etcd membership. This is valid behavior, so we capture the learner's
		// state and adapt the test accordingly.
		leaderNode, learnerNode, learnerStarted := validateEtcdRecoveryStateWithoutAssumingLeader(oc, etcdClientFactory,
			&peerNode, &targetNode, memberIsLeaderTimeout, utils.FiveSecondPollInterval)

		if learnerStarted {
			g.GinkgoT().Printf("Learner node '%s' already started as learner\n", learnerNode.Name)
		} else {
			g.By(fmt.Sprintf("Ensuring '%s' rejoins as learner (timeout: %v)", learnerNode.Name, memberRejoinedLearnerTimeout))
			validateEtcdRecoveryState(oc, etcdClientFactory,
				leaderNode,
				learnerNode, true, true, // targetNode expected started == true, learner == true
				memberRejoinedLearnerTimeout, utils.FiveSecondPollInterval)
		}

		g.By(fmt.Sprintf("Ensuring learner node '%s' is promoted back as voting member (timeout: %v)", learnerNode.Name, memberPromotedVotingTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			leaderNode,
			learnerNode, true, false, // targetNode expected started == true, learner == false
			memberPromotedVotingTimeout, utils.FiveSecondPollInterval)
	})

	g.It("should recover from a double node failure (cold-boot) [Requires:HypervisorSSHConfig]", func() {
		// Note: In a double node failure both nodes have the same role, hence we
		// will call them just NodeA and NodeB
		nodeA := peerNode
		nodeB := targetNode
		c, vmA, vmB, err := setupMinimalTestEnvironment(oc, &nodeA, &nodeB)
		o.Expect(err).To(o.BeNil(), "Expected to setup test environment without error")

		dataPair := []vmNodePair{
			{vmA, nodeA.Name},
			{vmB, nodeB.Name},
		}

		deferDiagnosticsOnFailure(oc, etcdClientFactory, &c, []corev1.Node{nodeA, nodeB})
		defer restartVms(dataPair, c)

		g.By("Simulating double node failure: stopping both nodes' VMs")
		// First, stop all VMs
		for _, d := range dataPair {
			err := services.VirshDestroyVM(d.vm, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
			o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to stop VM %s (node: %s)", d.vm, d.node))
		}
		// Then, wait for all to reach shut off state
		for _, d := range dataPair {
			err := services.WaitForVMState(d.vm, services.VMStateShutOff, vmUngracefulShutdownTimeout, utils.FiveSecondPollInterval, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
			o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected VM %s (node: %s) to reach shut off state in %s timeout", d.vm, d.node, vmUngracefulShutdownTimeout))
		}

		g.By("Restarting both nodes")
		restartVms(dataPair, c)

		g.By(fmt.Sprintf("Waiting both etcd members to become healthy (timeout: %v)", membersHealthyAfterDoubleReboot))
		// Both nodes are expected to be healthy voting members. The order of nodes passed to the validation function does not matter.
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&nodeA,
			&nodeB, true, false,
			membersHealthyAfterDoubleReboot, utils.FiveSecondPollInterval)
	})

	g.It("should recover from double graceful node shutdown (cold-boot) [Requires:HypervisorSSHConfig]", func() {
		// Note: Both nodes are gracefully shut down, then both restart
		nodeA := peerNode
		nodeB := targetNode
		g.GinkgoT().Printf("Testing double node graceful shutdown for %s and %s\n", nodeA.Name, nodeB.Name)

		c, vmA, vmB, err := setupMinimalTestEnvironment(oc, &nodeA, &nodeB)
		o.Expect(err).To(o.BeNil(), "Expected to setup test environment without error")

		dataPair := []vmNodePair{
			{vmA, nodeA.Name},
			{vmB, nodeB.Name},
		}

		deferDiagnosticsOnFailure(oc, etcdClientFactory, &c, []corev1.Node{nodeA, nodeB})
		defer restartVms(dataPair, c)

		g.By(fmt.Sprintf("Gracefully shutting down both nodes at the same time (timeout: %v)", vmGracefulShutdownTimeout))
		for _, d := range dataPair {
			innerErr := services.VirshShutdownVM(d.vm, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
			o.Expect(innerErr).To(o.BeNil(), fmt.Sprintf("Expected to gracefully shutdown VM %s (node: %s)", d.vm, d.node))
		}

		for _, d := range dataPair {
			innerErr := services.WaitForVMState(d.vm, services.VMStateShutOff, vmGracefulShutdownTimeout, utils.FiveSecondPollInterval, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
			o.Expect(innerErr).To(o.BeNil(), fmt.Sprintf("Expected VM %s (node: %s) to reach shut off state", d.vm, d.node))
		}

		g.By("Restarting both nodes")
		restartVms(dataPair, c)

		g.By(fmt.Sprintf("Waiting both etcd members to become healthy (timeout: %v)", membersHealthyAfterDoubleReboot))
		// Both nodes are expected to be healthy voting members. The order of nodes passed to the validation function does not matter.
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&nodeA,
			&nodeB, true, false,
			membersHealthyAfterDoubleReboot, utils.FiveSecondPollInterval)
	})

	g.It("should recover from sequential graceful node shutdowns (cold-boot) [Requires:HypervisorSSHConfig]", func() {
		// Note: First node is gracefully shut down, then the second, then both restart
		firstToShutdown := peerNode
		secondToShutdown := targetNode
		g.GinkgoT().Printf("Testing sequential graceful shutdowns: first %s, then %s\n",
			firstToShutdown.Name, secondToShutdown.Name)

		c, vmFirstToShutdown, vmSecondToShutdown, err := setupMinimalTestEnvironment(oc, &firstToShutdown, &secondToShutdown)
		o.Expect(err).To(o.BeNil(), "Expected to setup test environment without error")

		dataPair := []vmNodePair{
			{vmFirstToShutdown, firstToShutdown.Name},
			{vmSecondToShutdown, secondToShutdown.Name},
		}

		deferDiagnosticsOnFailure(oc, etcdClientFactory, &c, []corev1.Node{firstToShutdown, secondToShutdown})
		defer restartVms(dataPair, c)

		g.By(fmt.Sprintf("Gracefully shutting down first node: %s", firstToShutdown.Name))

		err = vmShutdownAndWait(VMShutdownModeGraceful, vmFirstToShutdown, c)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected VM %s to reach shut off state", vmFirstToShutdown))

		g.By(fmt.Sprintf("Gracefully shutting down second node: %s", secondToShutdown.Name))
		err = vmShutdownAndWait(VMShutdownModeGraceful, vmSecondToShutdown, c)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected VM %s to reach shut off state", vmSecondToShutdown))

		g.By("Restarting both nodes")
		restartVms(dataPair, c)

		g.By(fmt.Sprintf("Waiting both etcd members to become healthy (timeout: %v)", membersHealthyAfterDoubleReboot))
		// Both nodes are expected to be healthy voting members. The order of nodes passed to the validation function does not matter.
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&firstToShutdown,
			&secondToShutdown, true, false,
			membersHealthyAfterDoubleReboot, utils.FiveSecondPollInterval)
	})

	g.It("should recover from graceful shutdown followed by ungraceful node failure (cold-boot) [Requires:HypervisorSSHConfig]", func() {
		// Note: First node is gracefully shut down, then the survived node fails ungracefully
		firstToShutdown := targetNode
		secondToShutdown := peerNode
		g.GinkgoT().Printf("Randomly selected %s to shutdown gracefully and %s to survive, then fail ungracefully\n",
			firstToShutdown.Name, secondToShutdown.Name)

		c, vmFirstToShutdown, vmSecondToShutdown, err := setupMinimalTestEnvironment(oc, &firstToShutdown, &secondToShutdown)
		o.Expect(err).To(o.BeNil(), "Expected to setup test environment without error")

		dataPair := []vmNodePair{
			{vmFirstToShutdown, firstToShutdown.Name},
			{vmSecondToShutdown, secondToShutdown.Name},
		}

		deferDiagnosticsOnFailure(oc, etcdClientFactory, &c, []corev1.Node{firstToShutdown, secondToShutdown})
		defer restartVms(dataPair, c)

		g.By(fmt.Sprintf("Gracefully shutting down VM %s (node: %s)", vmFirstToShutdown, firstToShutdown.Name))
		err = vmShutdownAndWait(VMShutdownModeGraceful, vmFirstToShutdown, c)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected VM %s to reach shut off state", vmFirstToShutdown))

		g.By(fmt.Sprintf("Waiting for %s to recover the etcd cluster standalone (timeout: %v)", secondToShutdown.Name, memberIsLeaderTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&secondToShutdown,
			&firstToShutdown, false, true, // expected started == false, learner == true
			memberIsLeaderTimeout, utils.FiveSecondPollInterval)

		g.By(fmt.Sprintf("Ungracefully shutting down VM %s (node: %s)", vmSecondToShutdown, secondToShutdown.Name))
		err = vmShutdownAndWait(VMShutdownModeUngraceful, vmSecondToShutdown, c)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected VM %s to reach shut off state", vmSecondToShutdown))

		g.By("Restarting both nodes")
		restartVms(dataPair, c)

		g.By(fmt.Sprintf("Waiting both etcd members to become healthy (timeout: %v)", membersHealthyAfterDoubleReboot))
		// Both nodes are expected to be healthy voting members. The order of nodes passed to the validation function does not matter.
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&firstToShutdown,
			&secondToShutdown, true, false,
			membersHealthyAfterDoubleReboot, utils.FiveSecondPollInterval)
	})

	g.It("should recover from BMC credential rotation with fencing", func() {
		bmcNode := targetNode
		survivedNode := peerNode

		ns, secretName, originalPassword, err := apis.RotateNodeBMCPassword(oc, &bmcNode)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to rotate BMC credentials without error")

		defer func() {
			if err := apis.RestoreBMCPassword(oc, ns, secretName, originalPassword); err != nil {
				fmt.Fprintf(g.GinkgoWriter,
					"Warning: failed to restore original BMC password in %s/%s: %v\n",
					ns, secretName, err)
			}
		}()
		g.By("Ensuring etcd members remain healthy after BMC credential rotation")
		o.Eventually(func() error {
			if err := helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, survivedNode.Name); err != nil {
				return err
			}
			if err := helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, bmcNode.Name); err != nil {
				return err
			}
			return nil
		}, nodeIsHealthyTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(), "etcd members should be healthy after BMC credential rotation")

		g.By(fmt.Sprintf("Triggering a fencing-style network disruption between %s and %s", bmcNode.Name, survivedNode.Name))
		command, err := exutil.TriggerNetworkDisruption(oc.KubeClient(), &bmcNode, &survivedNode, networkDisruptionDuration)
		o.Expect(err).To(o.BeNil(), "Expected to disrupt network without errors")
		framework.Logf("network disruption command: %q", command)

		g.By(fmt.Sprintf("Ensuring cluster recovery with proper leader/learner roles after BMC credential rotation + network disruption (timeout: %v)", memberIsLeaderTimeout))
		leaderNode, learnerNode, learnerStarted := validateEtcdRecoveryStateWithoutAssumingLeader(oc, etcdClientFactory,
			&survivedNode, &bmcNode, memberIsLeaderTimeout, utils.FiveSecondPollInterval)

		if learnerStarted {
			framework.Logf("Learner node %q already started as learner after disruption", learnerNode.Name)
		} else {
			g.By(fmt.Sprintf("Ensuring '%s' rejoins as learner (timeout: %v)", learnerNode.Name, memberRejoinedLearnerTimeout))
			validateEtcdRecoveryState(oc, etcdClientFactory,
				leaderNode,
				learnerNode, true, true,
				memberRejoinedLearnerTimeout, utils.FiveSecondPollInterval)
		}

		g.By(fmt.Sprintf("Ensuring learner node '%s' is promoted back as voting member (timeout: %v)", learnerNode.Name, memberPromotedVotingTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			leaderNode,
			learnerNode, true, false,
			memberPromotedVotingTimeout, utils.FiveSecondPollInterval)
	})

	g.It("should recover from etcd process crash", func() {
		// Note: This test kills the etcd process/container on one node to simulate
		// a process crash, testing Pacemaker's ability to detect and restart etcd
		recoveryNode := peerNode
		g.GinkgoT().Printf("Randomly selected %s (%s) for etcd process crash and %s (%s) as recovery node\n",
			targetNode.Name, targetNode.Status.Addresses[0].Address, recoveryNode.Name, recoveryNode.Status.Addresses[0].Address)

		g.By(fmt.Sprintf("Killing etcd process/container on %s", targetNode.Name))
		_, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, targetNode.Name, "openshift-etcd",
			"bash", "-c", "podman kill etcd 2>/dev/null")
		o.Expect(err).To(o.BeNil(), "Expected to kill etcd process without command errors")

		g.By("Waiting for cluster to recover - both nodes become started voting members")
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&recoveryNode,
			&targetNode, true, false, // targetNode expected started == true, learner == false
			6*time.Minute, 45*time.Second)
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

		// Capture per-node baseline log line counts before the disruptive action so
		// log assertions only consider lines emitted during this test.
		logBaselines := getPacemakerLogBaselines(oc, nodes)

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
		expectPacemakerLogFound(oc, nodes, activeCountLogPattern, "Active count log entries", logBaselines)

		g.By("Verifying no 'Unexpected active resource count' errors in pacemaker logs")
		for _, node := range nodes {
			errorOutput, logErr := getPacemakerLogGrep(oc, node.Name, unexpectedCountError, logBaselines[node.Name])
			o.Expect(logErr).ShouldNot(o.HaveOccurred(),
				fmt.Sprintf("Expected to read pacemaker log on %s", node.Name))
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

		// Capture per-node baseline log line counts before the disruptive action so
		// log assertions only consider lines emitted during this test.
		logBaselines := getPacemakerLogBaselines(oc, nodes)

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
		expectPacemakerLogFound(oc, nodes, stoppingResourcesLogPattern, "Stopping resources log entries", logBaselines)

		g.By("Verifying delay intervention was applied to prevent simultaneous member removal")
		expectPacemakerLogFound(oc, nodes, delayStopLogPattern, "Delay intervention log", logBaselines)

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
		output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
			oc, targetNode.Name, "openshift-etcd",
			"bash", "-c", "podman kill etcd 2>/dev/null; true")
		framework.Logf("Podman kill output: %s, err: %v", output, err)

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
			o.Expect(failedSection).To(o.ContainSubstring(node.Name),
				fmt.Sprintf("Expected Failed Resource Actions to show failure on %s for coordinated recovery", node.Name))
			framework.Logf("Coordinated failure confirmed: node %s found in Failed Resource Actions", node.Name)
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

		// Verify learner_node attribute is set before we delete it.
		// The attribute is set by the leader's monitor action which runs asynchronously
		// after detecting the learner, so poll until it appears.
		g.By("Verifying learner_node CRM attribute is set")
		var attrOutput string
		o.Eventually(func() error {
			var queryErr error
			attrOutput, queryErr = utils.QueryCRMAttribute(oc, execNode.Name, crmAttributeName)
			return queryErr
		}, etcdResourceRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(
			o.HaveOccurred(), "Expected learner_node attribute to exist after force-new-cluster recovery")
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

func validateEtcdRecoveryState(
	oc *exutil.CLI, e *helpers.EtcdClientFactoryImpl,
	survivedNode, targetNode *corev1.Node,
	isTargetNodeStartedExpected, isTargetNodeLearnerExpected bool,
	timeout, pollInterval time.Duration,
) {
	attemptCount := 0
	lastLoggedAttempt := 0
	logEveryNAttempts := computeLogInterval(pollInterval)

	o.EventuallyWithOffset(1, func() error {
		attemptCount++
		shouldLog := attemptCount == 1 || (attemptCount-lastLoggedAttempt) >= logEveryNAttempts

		members, err := utils.GetMembers(e)
		if err != nil {
			if shouldLog {
				g.GinkgoT().Logf("[Attempt %d] Failed to get etcd members: %v", attemptCount, err)
				lastLoggedAttempt = attemptCount
			}
			return err
		}
		if len(members) != 2 {
			if shouldLog {
				g.GinkgoT().Logf("[Attempt %d] Expected 2 etcd members, got %d: %+v", attemptCount, len(members), members)
				lastLoggedAttempt = attemptCount
			}
			return fmt.Errorf("expected 2 members, got %d", len(members))
		}

		if isStarted, isLearner, err := utils.GetMemberState(survivedNode, members); err != nil {
			if shouldLog {
				g.GinkgoT().Logf("[Attempt %d] Failed to get state for survived node %s: %v", attemptCount, survivedNode.Name, err)
				lastLoggedAttempt = attemptCount
			}
			return err
		} else if !isStarted || isLearner {
			if shouldLog {
				g.GinkgoT().Logf("[Attempt %d] Survived node %s not ready: started=%v, learner=%v, members: %+v",
					attemptCount, survivedNode.Name, isStarted, isLearner, members)
				lastLoggedAttempt = attemptCount
			}
			return fmt.Errorf("expected survived node %s to be started and voting member, got this membership instead: %+v",
				survivedNode.Name, members)
		}

		isStarted, isLearner, err := utils.GetMemberState(targetNode, members)
		if err != nil {
			if shouldLog {
				g.GinkgoT().Logf("[Attempt %d] Failed to get state for target node %s: %v", attemptCount, targetNode.Name, err)
				lastLoggedAttempt = attemptCount
			}
			return err
		}

		// lazy check node reboot: make API calls only if and when needed
		var hasTargetNodeRebooted bool
		lazyCheckReboot := func() bool {
			// return cached value only if the node has already rebooted during this test
			if !hasTargetNodeRebooted {
				var checkErr error
				hasTargetNodeRebooted, checkErr = utils.HasNodeRebooted(oc, targetNode)
				if checkErr != nil {
					// Return false on error; Eventually will retry this entire validation function
					g.GinkgoT().Logf("Warning: failed to check reboot status: %v", checkErr)
					return false
				}
			}

			return hasTargetNodeRebooted
		}

		// NOTE: Target node restart, and also promotion to a voting member, can happen fast, and the
		// test might not be as quick as to get an etcd client and observe the intermediate states.
		// However, the fact that the targetNode rebooted proves disruption occurred as well,
		// and being its etcd member "started" and "voter" proves the recovery was successful.
		if isTargetNodeStartedExpected != isStarted {
			if !isTargetNodeStartedExpected && lazyCheckReboot() { // expected un-started, but started already after a reboot
				g.GinkgoT().Logf("[Attempt %d] Target node %s has re-started already", attemptCount, targetNode.Name)
			} else {
				if shouldLog {
					g.GinkgoT().Logf("[Attempt %d] Target node %s started state mismatch: expected=%v, got=%v, members: %+v",
						attemptCount, targetNode.Name, isTargetNodeStartedExpected, isStarted, members)
					lastLoggedAttempt = attemptCount
				}
				return fmt.Errorf("expected target node %s to have status started==%v (got %v). Full membership: %+v",
					targetNode.Name, isTargetNodeStartedExpected, isStarted, members)
			}
		}
		if isTargetNodeLearnerExpected != isLearner {
			if isTargetNodeLearnerExpected && lazyCheckReboot() { // expected "learner", but "voter" already after a reboot
				g.GinkgoT().Logf("[Attempt %d] Target node %s was promoted to voter already", attemptCount, targetNode.Name)
			} else {
				if shouldLog {
					g.GinkgoT().Logf("[Attempt %d] Target node %s learner state mismatch: expected=%v, got=%v, members: %+v",
						attemptCount, targetNode.Name, isTargetNodeLearnerExpected, isLearner, members)
					lastLoggedAttempt = attemptCount
				}
				return fmt.Errorf("expected target node %s to have status started==%v (got %v) and voting member==%v (got %v). Full membership: %+v",
					targetNode.Name, isTargetNodeStartedExpected, isStarted, isTargetNodeLearnerExpected, isLearner, members)
			}
		}

		g.GinkgoT().Logf("[Attempt %d] SUCCESS: etcd recovery validated, membership: %+v", attemptCount, members)
		return nil
	}, timeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred())
}

func validateEtcdRecoveryStateWithoutAssumingLeader(
	oc *exutil.CLI, e *helpers.EtcdClientFactoryImpl,
	nodeA, nodeB *corev1.Node,
	timeout, pollInterval time.Duration,
) (leaderNode, learnerNode *corev1.Node, learnerStarted bool) {
	attemptCount := 0
	lastLoggedAttempt := 0
	logEveryNAttempts := computeLogInterval(pollInterval)

	o.EventuallyWithOffset(1, func() error {
		attemptCount++
		shouldLog := attemptCount == 1 || (attemptCount-lastLoggedAttempt) >= logEveryNAttempts

		members, err := utils.GetMembers(e)
		if err != nil {
			if shouldLog {
				g.GinkgoT().Logf("[Attempt %d] Failed to get etcd members: %v", attemptCount, err)
				lastLoggedAttempt = attemptCount
			}
			return err
		}
		if len(members) != 2 {
			if shouldLog {
				g.GinkgoT().Logf("[Attempt %d] Expected 2 etcd members, got %d: %+v", attemptCount, len(members), members)
				lastLoggedAttempt = attemptCount
			}
			return fmt.Errorf("expected 2 members, got %d", len(members))
		}

		// Get state for both nodes first
		startedA, learnerA, err := utils.GetMemberState(nodeA, members)
		if err != nil {
			if shouldLog {
				g.GinkgoT().Logf("[Attempt %d] Failed to get state for node %s: %v", attemptCount, nodeA.Name, err)
				lastLoggedAttempt = attemptCount
			}
			return fmt.Errorf("failed to get state for node %s: %v", nodeA.Name, err)
		}

		startedB, learnerB, err := utils.GetMemberState(nodeB, members)
		if err != nil {
			if shouldLog {
				g.GinkgoT().Logf("[Attempt %d] Failed to get state for node %s: %v", attemptCount, nodeB.Name, err)
				lastLoggedAttempt = attemptCount
			}
			return fmt.Errorf("failed to get state for node %s: %v", nodeB.Name, err)
		}

		// Then, evaluate the possible combinations
		if !startedA && !startedB {
			if shouldLog {
				g.GinkgoT().Logf("[Attempt %d] Etcd members have not started yet: %s(started=%v), %s(started=%v)",
					attemptCount, nodeA.Name, startedA, nodeB.Name, startedB)
				lastLoggedAttempt = attemptCount
			}
			return fmt.Errorf("etcd members have not started yet")
		}

		// This should not happen
		if learnerA && learnerB {
			g.GinkgoT().Logf("[Attempt %d] ERROR: Both nodes are learners! %s(started=%v, learner=%v), %s(started=%v, learner=%v)",
				attemptCount, nodeA.Name, startedA, learnerA, nodeB.Name, startedB, learnerB)
			o.Expect(fmt.Errorf("both nodes are learners! %s(started=%v, learner=%v), %s(started=%v, learner=%v)",
				nodeA.Name, startedA, learnerA, nodeB.Name, startedB, learnerB)).ToNot(o.HaveOccurred())
		}

		// This might happen if the disruption didn't occurred yet, or we get this snapshot when the learner has been already promoted
		if !learnerA && !learnerB {
			// the disrupted node might have been promoted already due to fast promotion.
			// The promotion from learner to voting member can happen faster than the time
			// it takes us to establish an etcd client connection to the new etcd cluster
			// created by the survivedNode, and query the cluster state.
			// If one node rebooted, it proves a disruption occurred and recovery was successful,
			// even though we missed observing the intermediate learner state.
			hasNodeARebooted, err := utils.HasNodeRebooted(oc, nodeA)
			if err != nil {
				if shouldLog {
					g.GinkgoT().Logf("[Attempt %d] Failed to check reboot status for node %s: %v", attemptCount, nodeA.Name, err)
					lastLoggedAttempt = attemptCount
				}
				return err
			}
			hasNodeBRebooted, err := utils.HasNodeRebooted(oc, nodeB)
			if err != nil {
				if shouldLog {
					g.GinkgoT().Logf("[Attempt %d] Failed to check reboot status for node %s: %v", attemptCount, nodeB.Name, err)
					lastLoggedAttempt = attemptCount
				}
				return err
			}

			if hasNodeARebooted != hasNodeBRebooted {
				g.GinkgoT().Logf("[Attempt %d] Both nodes are non-learners, but only one has rebooted, hence the cluster has indeed recovered from a disruption", attemptCount)
				// the rebooted node is the learner
				learnerA = hasNodeARebooted
				learnerB = hasNodeBRebooted
			} else if hasNodeARebooted && hasNodeBRebooted {
				if shouldLog {
					g.GinkgoT().Logf("[Attempt %d] Both nodes rebooted - unexpected cluster disruption", attemptCount)
					lastLoggedAttempt = attemptCount
				}
				return fmt.Errorf("both nodes rebooted. This indicates a cluster disruption beyond the expected single-node failure")
			} else {
				if shouldLog {
					g.GinkgoT().Logf("[Attempt %d] Both nodes are non-learners: %s(started=%v, learner=%v), %s(started=%v, learner=%v)",
						attemptCount, nodeA.Name, startedA, learnerA, nodeB.Name, startedB, learnerB)
					lastLoggedAttempt = attemptCount
				}
				return fmt.Errorf("both nodes are non-learners (should have exactly one learner): %s(started=%v, learner=%v), %s(started=%v, learner=%v)", nodeA.Name, startedA, learnerA, nodeB.Name, startedB, learnerB)
			}
		}

		// Once we get one leader and one learner, we don't care if the latter has started already, but the first must
		// already been started
		leaderStarted := (startedA && !learnerA) || (startedB && !learnerB)
		if !leaderStarted {
			if shouldLog {
				g.GinkgoT().Logf("[Attempt %d] Leader node is not started: %s(started=%v, learner=%v), %s(started=%v, learner=%v)",
					attemptCount, nodeA.Name, startedA, learnerA, nodeB.Name, startedB, learnerB)
				lastLoggedAttempt = attemptCount
			}
			return fmt.Errorf("leader node is not started: %s(started=%v, learner=%v), %s(started=%v, learner=%v)",
				nodeA.Name, startedA, learnerA, nodeB.Name, startedB, learnerB)
		}

		// Set return values based on actual roles
		if learnerA {
			leaderNode = nodeB
			learnerNode = nodeA
			learnerStarted = startedA
		} else {
			leaderNode = nodeA
			learnerNode = nodeB
			learnerStarted = startedB
		}

		g.GinkgoT().Logf("[Attempt %d] SUCCESS: Leader is %s, learner is %s (started=%v)",
			attemptCount, leaderNode.Name, learnerNode.Name, learnerStarted)

		return nil
	}, timeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred())

	return leaderNode, learnerNode, learnerStarted
}

// setupMinimalTestEnvironment validates prerequisites and gathers required information for double node failure test
func setupMinimalTestEnvironment(oc *exutil.CLI, nodeA, nodeB *corev1.Node) (c hypervisorExtendedConfig, vmNameNodeA, vmNameNodeB string, err error) {
	if !exutil.HasHypervisorConfig() {
		services.PrintHypervisorConfigUsage()
		err = fmt.Errorf("no hypervisor configuration available")
		return
	}

	sshConfig := exutil.GetHypervisorConfig()
	c.HypervisorConfig.IP = sshConfig.HypervisorIP
	c.HypervisorConfig.User = sshConfig.SSHUser
	c.HypervisorConfig.PrivateKeyPath = sshConfig.PrivateKeyPath

	// Validate that the private key file exists
	if _, err = os.Stat(c.HypervisorConfig.PrivateKeyPath); os.IsNotExist(err) {
		return
	}

	c.HypervisorKnownHostsPath, err = core.PrepareLocalKnownHostsFile(&c.HypervisorConfig)
	if err != nil {
		return
	}

	err = services.VerifyHypervisorAvailability(&c.HypervisorConfig, c.HypervisorKnownHostsPath)
	if err != nil {
		return
	}

	// This assumes VMs are named similarly to the OpenShift nodes (e.g., master-0, master-1)
	vmNameNodeA, err = services.FindVMByNodeName(nodeA.Name, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
	if err != nil {
		err = fmt.Errorf("failed to find node's %s VM: %w", nodeA.Name, err)
		return
	}

	vmNameNodeB, err = services.FindVMByNodeName(nodeB.Name, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
	if err != nil {
		err = fmt.Errorf("failed to find node's %s VM: %w", nodeB.Name, err)
		return
	}

	return
}

type vmNodePair struct {
	vm, node string
}

type VMShutdownMode int

const (
	VMShutdownModeGraceful VMShutdownMode = iota + 1
	VMShutdownModeUngraceful
)

func (sm VMShutdownMode) String() string {
	switch sm {
	case VMShutdownModeGraceful:
		return "graceful VM shutdown"
	case VMShutdownModeUngraceful:
		return "ungraceful VM shutdown"
	}
	return "unknown vm shutdown mode"
}

func vmShutdownAndWait(mode VMShutdownMode, vm string, c hypervisorExtendedConfig) error {
	var timeout time.Duration
	var shutdownFunc func(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) error
	switch mode {
	case VMShutdownModeGraceful:
		timeout = vmGracefulShutdownTimeout
		shutdownFunc = services.VirshShutdownVM
	case VMShutdownModeUngraceful:
		timeout = vmUngracefulShutdownTimeout
		shutdownFunc = services.VirshDestroyVM
	default:
		return fmt.Errorf("unexpected VMShutdownMode: %s", mode)
	}

	g.GinkgoT().Printf("%s: vm %s (timeout: %v)\n", mode, vm, timeout)
	err := shutdownFunc(vm, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
	if err != nil {
		return err
	}

	return services.WaitForVMState(vm, services.VMStateShutOff, timeout, utils.FiveSecondPollInterval, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
}

// restartVms starts all VMs asynchronously, then wait for them to be running
func restartVms(dataPair []vmNodePair, c hypervisorExtendedConfig) {
	var restartedVms []vmNodePair
	// Start all VMs asynchronously
	for _, d := range dataPair {
		state, err := services.GetVMState(d.vm, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "Warning: cleanup failed to check VM '%s' state: %v\n", d.vm, err)
			fmt.Fprintf(g.GinkgoWriter, "Trying to start VM '%s' anyway\n", d.vm)
			state = services.VMStateShutOff
		}

		if state == services.VMStateShutOff {
			if err = services.VirshStartVM(d.vm, &c.HypervisorConfig, c.HypervisorKnownHostsPath); err != nil {
				fmt.Fprintf(g.GinkgoWriter, "Warning: failed to restart VM %s during cleanup: %v\n", d.vm, err)
				continue
			}
			restartedVms = append(restartedVms, d)
		}
	}

	// Wait for all VMs to be running
	for _, d := range restartedVms {
		err := services.WaitForVMState(d.vm, services.VMStateRunning, vmRestartTimeout, utils.FiveSecondPollInterval, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected VM %s (node: %s) to start in %s timeout", d.vm, d.node, vmRestartTimeout))
	}
}

// deferDiagnosticsOnFailure registers a DeferCleanup handler that gathers diagnostic
// information when the current test spec fails. This should be called early in test
// setup to ensure diagnostics are collected on any failure.
//
//	deferDiagnosticsOnFailure(oc, etcdClientFactory, &c, []corev1.Node{nodeA, nodeB})
func deferDiagnosticsOnFailure(
	oc *exutil.CLI,
	etcdClientFactory *helpers.EtcdClientFactoryImpl,
	c *hypervisorExtendedConfig,
	nodes []corev1.Node,
) {
	g.DeferCleanup(func() {
		if g.CurrentSpecReport().Failed() {
			gatherRecoveryDiagnostics(oc, etcdClientFactory, c, nodes)
		}
	})
}

// gatherRecoveryDiagnostics collects diagnostic information when a recovery test fails.
// This gathers:
// 1. VM states from the hypervisor (virsh list --all)
// 2. Pacemaker status from both nodes (pcs status --full)
// 3. etcd member list from both nodes
//
// This helps diagnose why etcd recovery failed by showing:
// - Whether VMs are actually running
// - Pacemaker cluster state and any fencing issues
// - Current etcd membership and learner/voting status
func gatherRecoveryDiagnostics(
	oc *exutil.CLI,
	etcdClientFactory *helpers.EtcdClientFactoryImpl,
	c *hypervisorExtendedConfig,
	nodes []corev1.Node,
) {
	framework.Logf("========== GATHERING RECOVERY DIAGNOSTICS ==========")

	var gatherErrors []string

	// 1. Get VM states from hypervisor
	framework.Logf("--- VM States from Hypervisor ---")
	if vmList, err := services.VirshList(&c.HypervisorConfig, c.HypervisorKnownHostsPath, services.VirshListFlagAll); err != nil {
		gatherErrors = append(gatherErrors, fmt.Sprintf("VM list: %v", err))
	} else {
		framework.Logf("virsh list --all output:\n%s", vmList)
	}

	// 2. Get pcs status --full from each node (try both, use first that succeeds)
	framework.Logf("--- Pacemaker Status ---")
	pcsStatusGathered := false
	for _, node := range nodes {
		nodeIP := utils.GetNodeInternalIP(&node)
		if nodeIP == "" {
			continue
		}

		// Get the remote known hosts path for this node
		remoteKnownHostsPath, err := core.PrepareRemoteKnownHostsFile(nodeIP, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
		if err != nil {
			continue
		}

		pcsOutput, _, err := services.PcsStatusFull(nodeIP, &c.HypervisorConfig, c.HypervisorKnownHostsPath, remoteKnownHostsPath)
		if err != nil {
			continue
		}
		framework.Logf("pcs status --full from node %s:\n%s", node.Name, pcsOutput)
		pcsStatusGathered = true
		break // Only need one successful pcs status
	}
	if !pcsStatusGathered {
		gatherErrors = append(gatherErrors, "pcs status: could not gather from any node")
	}

	// 3. Get etcd member list
	framework.Logf("--- etcd Member List ---")
	etcdClient, closeFn, err := etcdClientFactory.NewEtcdClient()
	if err != nil {
		gatherErrors = append(gatherErrors, fmt.Sprintf("etcd client: %v", err))
	} else {
		defer closeFn()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		memberList, err := etcdClient.MemberList(ctx)
		if err != nil {
			gatherErrors = append(gatherErrors, fmt.Sprintf("etcd member list: %v", err))
		} else {
			framework.Logf("etcd members:")
			for _, member := range memberList.Members {
				learnerStatus := "voting"
				if member.IsLearner {
					learnerStatus = "learner"
				}
				startedStatus := "not started"
				if member.Name != "" {
					startedStatus = "started"
				}
				framework.Logf("  - %s (ID: %x): %s, %s, PeerURLs: %v, ClientURLs: %v",
					member.Name, member.ID, learnerStatus, startedStatus, member.PeerURLs, member.ClientURLs)
			}
		}
	}

	// Log summary of any errors encountered during diagnostics gathering
	if len(gatherErrors) > 0 {
		framework.Logf("--- Diagnostics Gathering Errors ---")
		framework.Logf("Some diagnostics could not be gathered: %v", gatherErrors)
	}

	framework.Logf("========== END RECOVERY DIAGNOSTICS ==========")
}
