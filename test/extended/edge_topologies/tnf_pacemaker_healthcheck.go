package edge_topologies

import (
	"fmt"
	"math/rand"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/edge_topologies/utils"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/apis"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/services"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const healthCheckRecoveryTimeout = 10 * time.Minute

// findStonithResourceName determines the pcs stonith resource name that fences
// targetNode (e.g. "master-0_redfish") via the PacemakerCluster CR — the object
// under test, giving a deterministic Name/Method lookup. The BeforeEach in this
// suite skips the test entirely when the CR is unavailable, so callers can rely
// on it being present here.
func findStonithResourceName(oc *exutil.CLI, targetNode *corev1.Node) (string, error) {
	pc, err := apis.GetPacemakerCluster(oc)
	if err != nil {
		return "", err
	}
	return apis.FindStartedFencingAgent(pc, targetNode.Name)
}

// waitForHealthCheckClearedBestEffort blocks (bounded) until PacemakerHealthCheckDegraded
// clears before a DeferCleanup returns. Without this, a spec that fails before reaching its
// own WaitForPacemakerHealthCheckCleared call restores pcs state via a best-effort helper that
// returns immediately, leaving Degraded=True; the next spec's BeforeEach then observes the
// still-True condition via SkipIfClusterIsNotHealthy's one-shot check and skips instead of
// running against a genuinely broken pipeline. Logs rather than fails on timeout, consistent
// with the best-effort restores it follows.
func waitForHealthCheckClearedBestEffort(oc *exutil.CLI) {
	if err := apis.WaitForPacemakerHealthCheckCleared(oc, healthCheckRecoveryTimeout); err != nil {
		framework.Logf("DeferCleanup: PacemakerHealthCheckDegraded did not clear: %v", err)
	}
}

// deferHealthCheckDiagnosticsOnFailure registers a DeferCleanup that dumps
// PacemakerHealthCheck diagnostics when the current spec fails. WaitForPacemakerHealthCheckDegraded
// and WaitForPacemakerHealthCheckCleared already dump diagnostics on their own timeout, but any
// other failing assertion in this suite (e.g. an Eventually/Consistently on the PacemakerCluster
// CR, or a WaitForPacemakerEvent timeout) would otherwise leave a failure with no diagnostics at
// all. Mirrors deferDiagnosticsOnFailure in tnf_recovery.go.
func deferHealthCheckDiagnosticsOnFailure(oc *exutil.CLI) {
	g.DeferCleanup(func() {
		if g.CurrentSpecReport().Failed() {
			apis.DumpHealthCheckDiagnostics(oc, "spec failed")
		}
	})
}

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Serial][Disruptive] PacemakerHealthCheck degraded condition", func() {
	defer g.GinkgoRecover()

	var (
		oc                = exutil.NewCLIWithoutNamespace("").AsAdmin()
		etcdClientFactory *helpers.EtcdClientFactoryImpl
		execNode          corev1.Node
		targetNode        corev1.Node
		nodes             []corev1.Node
	)

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)

		etcdClientFactory = helpers.NewEtcdClientFactory(oc.KubeClient())

		utils.SkipIfClusterIsNotHealthy(oc, etcdClientFactory)

		hasPacemakerCR, availErr := apis.IsPacemakerClusterAvailable(oc)
		o.Expect(availErr).ToNot(o.HaveOccurred(), "expected to check PacemakerCluster availability without error")
		if !hasPacemakerCR {
			g.Skip("PacemakerCluster CRD not available")
		}

		nodeList, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).To(o.BeNil(), "Expected to retrieve nodes without error")
		o.Expect(len(nodeList.Items)).To(o.Equal(2), "Expected exactly 2 nodes for two-node cluster")

		randomIndex := rand.Intn(len(nodeList.Items))
		execNode = nodeList.Items[randomIndex]
		targetNode = nodeList.Items[(randomIndex+1)%len(nodeList.Items)]
		nodes = nodeList.Items

		deferHealthCheckDiagnosticsOnFailure(oc)

		g.DeferCleanup(func() {
			logFinalClusterStatus(nodes)
		})
	})

	g.It("should detect and recover from cluster maintenance mode", func() {
		g.By("Verifying PacemakerCluster CR baseline is fully healthy before the test")
		o.Expect(apis.ExpectPacemakerBaseline(oc)).ToNot(o.HaveOccurred(), "expected PacemakerCluster to be fully healthy before test")

		// Capture an event baseline before the disruptive action so the event
		// assertions below only accept freshly-emitted events, not stale ones
		// left over from a prior reconcile or test run.
		maintenanceBaseline := time.Now()

		g.By("Enabling cluster maintenance mode")
		err := services.PcsPropertySetViaDebug(oc, execNode.Name, "maintenance-mode", "true")
		o.Expect(err).To(o.BeNil(), "Expected to enable maintenance mode")

		g.DeferCleanup(func() {
			framework.Logf("DeferCleanup: Ensuring maintenance mode is disabled")
			services.PcsPropertySetBestEffortViaDebug(oc, execNode.Name, "maintenance-mode", "false")
			waitForHealthCheckClearedBestEffort(oc)
		})

		g.By("Waiting for PacemakerHealthCheckDegraded=True due to maintenance mode")
		o.Expect(apis.WaitForPacemakerHealthCheckDegraded(oc, "maintenance mode", healthCheckRecoveryTimeout)).
			ShouldNot(o.HaveOccurred(), "PacemakerHealthCheckDegraded should become True when cluster is in maintenance mode")

		g.By("Verifying PacemakerClusterInMaintenance event was emitted")
		o.Expect(apis.WaitForPacemakerEvent(oc, apis.PacemakerHealthCheckEventNamespace, "PacemakerClusterInMaintenance", maintenanceBaseline, 2*time.Minute)).
			ShouldNot(o.HaveOccurred(), "Expected PacemakerClusterInMaintenance event in openshift-etcd-operator namespace")

		// Baseline for the post-recovery PacemakerHealthy event: the cluster was
		// healthy before this test, so an earlier PacemakerHealthy event may
		// exist. Require one emitted after recovery begins.
		recoveryBaseline := time.Now()

		g.By("Disabling cluster maintenance mode")
		err = services.PcsPropertySetViaDebug(oc, execNode.Name, "maintenance-mode", "false")
		o.Expect(err).To(o.BeNil(), "Expected to disable maintenance mode")

		g.By("Waiting for PacemakerHealthCheckDegraded to clear")
		o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, healthCheckRecoveryTimeout)).
			ShouldNot(o.HaveOccurred(), "PacemakerHealthCheckDegraded should clear after maintenance mode is disabled")

		g.By("Verifying PacemakerHealthy event was emitted after recovery")
		o.Expect(apis.WaitForPacemakerEvent(oc, apis.PacemakerHealthCheckEventNamespace, "PacemakerHealthy", recoveryBaseline, 2*time.Minute)).
			ShouldNot(o.HaveOccurred(), "Expected PacemakerHealthy event in openshift-etcd-operator namespace after recovery")

		g.By("Validating cluster health after maintenance mode recovery")
		o.Eventually(func() error {
			return utils.ValidateEssentialOperatorsAvailable(oc)
		}, healthCheckRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(
			o.HaveOccurred(), "Essential operators should be available after maintenance mode recovery")

		g.By("Verifying PacemakerCluster CR baseline is fully healthy after the test")
		o.Eventually(func() error {
			return apis.ExpectPacemakerBaseline(oc)
		}, healthCheckRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(
			o.HaveOccurred(), "expected PacemakerCluster to be fully healthy after test")
	})

	g.It("should detect and recover from a node going offline via pcs cluster stop", func() {
		g.By("Verifying PacemakerCluster CR baseline is fully healthy before the test")
		o.Expect(apis.ExpectPacemakerBaseline(oc)).ToNot(o.HaveOccurred(), "expected PacemakerCluster to be fully healthy before test")

		g.By(fmt.Sprintf("Stopping pacemaker cluster on %s from peer node %s", targetNode.Name, execNode.Name))
		err := services.PcsClusterStopViaDebug(oc, execNode.Name, targetNode.Name)
		o.Expect(err).To(o.BeNil(), "Expected pcs cluster stop to succeed")

		g.DeferCleanup(func() {
			framework.Logf("DeferCleanup: Ensuring pacemaker cluster is started on %s", targetNode.Name)
			services.PcsClusterStartBestEffortViaDebug(oc, execNode.Name, targetNode.Name)
			waitForHealthCheckClearedBestEffort(oc)
		})

		// Accept any degraded message rather than requiring "is offline". Even in
		// this single-node-down scenario the controller can reach degraded via the
		// staleness path (message "is stale") if the status-collector CronJob pod is
		// scheduled onto the corosync-stopped node before the surviving node's
		// collector writes NodeOnline=False. Both paths correctly signal degradation.
		g.By("Waiting for PacemakerHealthCheckDegraded=True due to node offline")
		o.Expect(apis.WaitForPacemakerHealthCheckDegraded(oc, "", apis.PacemakerDegradedDetectionTimeout)).
			ShouldNot(o.HaveOccurred(), "PacemakerHealthCheckDegraded should become True when a node is offline")

		// NodeCountAsExpected is derived from the CIB (`pcs cluster config`), which
		// still lists both nodes after `pcs cluster stop` — stopping corosync on a
		// node does not remove it from the configured node count. The condition must
		// therefore remain True while the node is offline.
		g.By("Verifying PacemakerCluster CR keeps NodeCountAsExpected=True while node is offline")
		o.Eventually(func() error {
			pc, pcErr := apis.GetPacemakerCluster(oc)
			if pcErr != nil {
				return pcErr
			}
			return apis.ExpectClusterNodeCountAsExpected(pc)
		}, 2*time.Minute, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(),
			"NodeCountAsExpected should remain True while node is offline (pcs cluster stop does not change the CIB node count)")

		g.By(fmt.Sprintf("Starting pacemaker cluster on %s", targetNode.Name))
		err = services.PcsClusterStartViaDebug(oc, execNode.Name, targetNode.Name)
		o.Expect(err).To(o.BeNil(), "Expected pcs cluster start to succeed")

		g.By("Waiting for PacemakerHealthCheckDegraded to clear")
		o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, healthCheckRecoveryTimeout)).
			ShouldNot(o.HaveOccurred(), "PacemakerHealthCheckDegraded should clear after node comes back online")

		g.By("Validating cluster health after node restart")
		o.Eventually(func() error {
			return utils.ValidateEssentialOperatorsAvailable(oc)
		}, healthCheckRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(
			o.HaveOccurred(), "Essential operators should be available after node restart")

		g.By("Validating etcd cluster recovered")
		o.Eventually(func() error {
			return utils.LogEtcdClusterStatus(oc, "after pcs cluster start", etcdClientFactory)
		}, healthCheckRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(
			o.HaveOccurred(), "Etcd cluster should be healthy after node restart")

		g.By("Verifying PacemakerCluster CR baseline is fully healthy after the test")
		o.Eventually(func() error {
			return apis.ExpectPacemakerBaseline(oc)
		}, healthCheckRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(
			o.HaveOccurred(), "expected PacemakerCluster to be fully healthy after test")
	})

	g.It("should not degrade when fencing is at risk but still available", func() {
		g.By("Verifying PacemakerCluster CR baseline is fully healthy before the test")
		o.Expect(apis.ExpectPacemakerBaseline(oc)).ToNot(o.HaveOccurred(), "expected PacemakerCluster to be fully healthy before test")

		g.By("Finding a fencing agent to unmanage on the target node")
		stonithResourceName, err := findStonithResourceName(oc, &targetNode)
		if err != nil {
			framework.Logf("Could not identify a started fencing agent for the target node: %v", err)
			g.Skip("Could not identify a started fencing agent for the target node — skipping negative test")
		}
		framework.Logf("Selected fencing agent to unmanage: %s", stonithResourceName)

		g.By(fmt.Sprintf("Unmanaging fencing agent %s to create FencingHealthy=False, FencingAvailable=True state", stonithResourceName))
		err = services.PcsStonithSetManagedViaDebug(oc, execNode.Name, stonithResourceName, false)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to unmanage fencing agent")

		g.DeferCleanup(func() {
			framework.Logf("Restoring management of fencing agent %s", stonithResourceName)
			services.PcsStonithSetManagedBestEffortViaDebug(oc, execNode.Name, stonithResourceName, true)
			waitForHealthCheckClearedBestEffort(oc)
		})

		g.By("Waiting for PacemakerCluster to report FencingHealthy=False, FencingAvailable=True for target node")
		o.Eventually(func() error {
			pc, pcErr := apis.GetPacemakerCluster(oc)
			if pcErr != nil {
				return pcErr
			}
			if err := apis.ExpectNodeFencingUnhealthy(pc, targetNode.Name); err != nil {
				return err
			}
			return apis.ExpectNodeFencingAvailable(pc, targetNode.Name)
		}, 2*time.Minute, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(),
			"expected fencing to be at-risk (FencingHealthy=False) but still available (FencingAvailable=True) for target node")

		g.By("Verifying PacemakerHealthCheckDegraded stays False during fencing warning state")
		o.Consistently(func() error {
			return apis.ExpectPacemakerHealthCheckNotDegraded(oc)
		}, 3*time.Minute, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(),
			"PacemakerHealthCheckDegraded should stay False when fencing is at risk but still available")

		g.By(fmt.Sprintf("Re-managing fencing agent %s", stonithResourceName))
		err = services.PcsStonithSetManagedViaDebug(oc, execNode.Name, stonithResourceName, true)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to re-manage fencing agent")

		g.By("Verifying PacemakerCluster CR baseline is fully healthy after re-managing fencing agent")
		o.Eventually(func() error {
			return apis.ExpectPacemakerBaseline(oc)
		}, fencingHealthTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(),
			"expected PacemakerCluster to be fully healthy after re-managing fencing agent")
	})

	g.It("should degrade when a node's fencing agent is completely unavailable", func() {
		g.By("Verifying PacemakerCluster CR baseline is fully healthy before the test")
		o.Expect(apis.ExpectPacemakerBaseline(oc)).ToNot(o.HaveOccurred(), "expected PacemakerCluster to be fully healthy before test")

		g.By("Finding a fencing agent to disable on the target node")
		stonithResourceName, err := findStonithResourceName(oc, &targetNode)
		if err != nil {
			framework.Logf("Could not identify a started fencing agent for the target node: %v", err)
			g.Skip("Could not identify a started fencing agent for the target node — skipping fencing disable test")
		}
		framework.Logf("Selected fencing agent to disable: %s", stonithResourceName)

		g.By(fmt.Sprintf("Disabling fencing agent %s to make fencing completely unavailable for %s", stonithResourceName, targetNode.Name))
		err = services.PcsStonithResourceDisableViaDebug(oc, execNode.Name, stonithResourceName)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to disable fencing agent")

		g.DeferCleanup(func() {
			framework.Logf("Re-enabling fencing agent %s", stonithResourceName)
			services.PcsStonithResourceEnableBestEffortViaDebug(oc, execNode.Name, stonithResourceName)
			waitForHealthCheckClearedBestEffort(oc)
		})

		g.By("Waiting for PacemakerHealthCheckDegraded=True due to fencing unavailable")
		o.Expect(apis.WaitForPacemakerHealthCheckDegraded(oc, "fencing unavailable", healthCheckRecoveryTimeout)).
			ShouldNot(o.HaveOccurred(), "PacemakerHealthCheckDegraded should become True when fencing is completely unavailable")

		g.By("Verifying PacemakerCluster CR shows FencingAvailable=False for target node")
		o.Eventually(func() error {
			pc, pcErr := apis.GetPacemakerCluster(oc)
			if pcErr != nil {
				return pcErr
			}
			return apis.ExpectNodeFencingUnavailable(pc, targetNode.Name)
		}, 2*time.Minute, 10*time.Second).ShouldNot(o.HaveOccurred(),
			"expected FencingAvailable=False on PacemakerCluster CR for target node")

		g.By(fmt.Sprintf("Re-enabling fencing agent %s", stonithResourceName))
		err = services.PcsStonithResourceEnableViaDebug(oc, execNode.Name, stonithResourceName)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to re-enable fencing agent")

		g.By("Waiting for PacemakerHealthCheckDegraded to clear after re-enabling fencing")
		o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, healthCheckRecoveryTimeout)).
			ShouldNot(o.HaveOccurred(), "PacemakerHealthCheckDegraded should clear after fencing is re-enabled")

		g.By("Verifying PacemakerCluster CR baseline is fully healthy after re-enabling fencing agent")
		o.Eventually(func() error {
			return apis.ExpectPacemakerBaseline(oc)
		}, healthCheckRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(),
			"expected PacemakerCluster to be fully healthy after re-enabling fencing agent")
	})
})
