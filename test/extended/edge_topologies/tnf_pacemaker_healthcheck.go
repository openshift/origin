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
// still-True condition via SkipIfPacemakerHealthCheckBaselineNotReady's one-shot check and skips
// instead of running against a genuinely broken pipeline. Logs rather than fails on timeout, consistent
// with the best-effort restores it follows.
func waitForHealthCheckClearedBestEffort(oc *exutil.CLI) {
	if err := apis.WaitForPacemakerHealthCheckCleared(oc, healthCheckRecoveryTimeout); err != nil {
		framework.Logf("DeferCleanup: PacemakerHealthCheckDegraded did not clear: %v", err)
	}
}

// checkPacemakerHealthyEventObserved performs a bounded, non-blocking, informational
// check for a PacemakerHealthy event emitted at or after since,
// since the event is informational and not required for recovery.
func checkPacemakerHealthyEventObserved(oc *exutil.CLI, since time.Time) {
	if err := apis.WaitForPacemakerEvent(oc, apis.PacemakerHealthCheckEventNamespace, "PacemakerHealthy", since, 2*time.Minute); err != nil {
		framework.Logf("[sig-etcd][PHCMiss] PacemakerHealthy event not observed for recovery starting %s: %v",
			since.Format(time.RFC3339), err)
	} else {
		framework.Logf("[sig-etcd][PHCCheck] PacemakerHealthy event observed for recovery starting %s",
			since.Format(time.RFC3339))
	}
}

func waitForFreshPacemakerClusterStatus(oc *exutil.CLI) {
	g.By("Waiting for a fresh PacemakerCluster status")
	o.Eventually(func() error {
		lastUpdated, err := apis.GetPacemakerClusterLastUpdated(oc)
		if err != nil {
			return err
		}
		if age := time.Since(lastUpdated); age >= time.Minute {
			return fmt.Errorf("PacemakerCluster lastUpdated is %s old, expected < 1m", age.Round(time.Second))
		}
		return nil
	}, 3*time.Minute, 10*time.Second).Should(o.Succeed(),
		"PacemakerCluster lastUpdated should be fresh before the disruptive action")
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

		utils.SkipIfPacemakerHealthCheckBaselineNotReady(oc)

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

		// CR-clock baseline for the fresh-snapshot wait below.
		preRecoveryPC, err := apis.GetPacemakerCluster(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "expected to fetch PacemakerCluster before recovery")

		g.By("Disabling cluster maintenance mode")
		err = services.PcsPropertySetViaDebug(oc, execNode.Name, "maintenance-mode", "false")
		o.Expect(err).To(o.BeNil(), "Expected to disable maintenance mode")

		g.By("Waiting for a fresh, healthy PacemakerCluster snapshot after disabling maintenance mode")
		o.Expect(apis.WaitForFreshHealthyPacemakerSnapshot(oc, preRecoveryPC.Status.LastUpdated.Time, healthCheckRecoveryTimeout)).
			ShouldNot(o.HaveOccurred(), "expected a fresh, healthy PacemakerCluster snapshot after disabling maintenance mode")

		g.By("Waiting for PacemakerHealthCheckDegraded to clear")
		o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, healthCheckRecoveryTimeout)).
			ShouldNot(o.HaveOccurred(), "PacemakerHealthCheckDegraded should clear after maintenance mode is disabled")

		g.By("Checking for PacemakerHealthy event after recovery (informational)")
		checkPacemakerHealthyEventObserved(oc, recoveryBaseline)

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

	g.It("should remain healthy while status-collector writes are blocked", func() {
		g.DeferCleanup(func() {
			if err := apis.UnblockStatusCollectorWrites(oc); err != nil {
				framework.Logf("DeferCleanup: failed to unblock status collector writes: %v", err)
			}
			waitForHealthCheckClearedBestEffort(oc)
		})

		waitForFreshPacemakerClusterStatus(oc)

		g.By("Capturing an event baseline before blocking status collector writes")
		baseline := time.Now()

		g.By("Blocking status collector writes for less than the staleness threshold")
		o.Expect(apis.BlockStatusCollectorWrites(oc)).To(o.Succeed(), "expected to block status collector writes")

		g.By("Waiting for the admission policy to deny collector writes")
		o.Expect(apis.WaitForStatusCollectorWritesBlocked(oc, 2*time.Minute)).To(o.Succeed(),
			"admission policy should deny status collector writes")

		g.By("Capturing the frozen PacemakerCluster status timestamp")
		frozenLastUpdated, err := apis.GetPacemakerClusterLastUpdated(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "expected to read frozen PacemakerCluster lastUpdated")
		remainingGracePeriod := 4*time.Minute - time.Since(frozenLastUpdated)
		o.Expect(remainingGracePeriod).To(o.BeNumerically(">", 0),
			"frozen PacemakerCluster lastUpdated must be younger than four minutes")

		// Assert the condition stays explicitly False for the whole grace period.
		// Checking only "not True" would let a missing or Unknown condition pass,
		// masking a controller that stopped reporting a healthy baseline while the
		// status collector is blocked (a false negative).
		g.By("Verifying PacemakerHealthCheckDegraded stays explicitly False until the frozen status is four minutes old")
		o.Consistently(func() error {
			return apis.ExpectPacemakerHealthCheckExplicitlyNotDegraded(oc)
		}, remainingGracePeriod, 10*time.Second).Should(o.Succeed(),
			"PacemakerHealthCheckDegraded should remain explicitly False until the frozen status is four minutes old")

		g.By("Unblocking status collector writes")
		o.Expect(apis.UnblockStatusCollectorWrites(oc)).To(o.Succeed(), "expected to unblock status collector writes")

		g.By("Verifying no stale status event was emitted during the grace period")
		o.Expect(apis.ExpectNoPacemakerEventSince(oc, apis.PacemakerHealthCheckEventNamespace, "PacemakerStatusStale", baseline)).
			To(o.Succeed(), "PacemakerStatusStale should not be emitted during the grace period")

		g.By("Waiting for PacemakerCluster status to become fresh for the next spec")
		o.Eventually(func() error {
			lastUpdated, err := apis.GetPacemakerClusterLastUpdated(oc)
			if err != nil {
				return err
			}
			if time.Since(lastUpdated) >= 3*time.Minute {
				return fmt.Errorf("PacemakerCluster lastUpdated is %s old, expected < 3m", time.Since(lastUpdated).Round(time.Second))
			}
			return nil
		}, 5*time.Minute, 10*time.Second).Should(o.Succeed(),
			"PacemakerCluster lastUpdated should become fresh after unblocking status collector writes")
	})

	g.It("should detect and recover from stale PacemakerCluster status", func() {
		waitForFreshPacemakerClusterStatus(oc)

		g.By("Capturing the initial PacemakerCluster status timestamp")
		initialLastUpdated, err := apis.GetPacemakerClusterLastUpdated(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "expected to read initial PacemakerCluster lastUpdated")

		g.DeferCleanup(func() {
			if err := apis.UnblockStatusCollectorWrites(oc); err != nil {
				framework.Logf("DeferCleanup: failed to unblock status collector writes: %v", err)
			}
			waitForHealthCheckClearedBestEffort(oc)
		})

		g.By("Blocking status collector writes")
		o.Expect(apis.BlockStatusCollectorWrites(oc)).To(o.Succeed(), "expected to block status collector writes")

		g.By("Waiting for the admission policy to deny collector writes")
		o.Expect(apis.WaitForStatusCollectorWritesBlocked(oc, 2*time.Minute)).To(o.Succeed(),
			"admission policy should deny status collector writes")

		g.By("Re-baselining PacemakerCluster status after the collector has settled")
		frozenLastUpdated, err := apis.GetPacemakerClusterLastUpdated(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "expected to read settled PacemakerCluster lastUpdated")
		o.Expect(frozenLastUpdated).To(o.BeTemporally(">=", initialLastUpdated), "lastUpdated should not move backwards")
		staleBaseline := time.Now()

		g.By("Verifying PacemakerCluster status stops advancing while status collector writes are blocked")
		o.Consistently(func() error {
			lastUpdated, err := apis.GetPacemakerClusterLastUpdated(oc)
			if err != nil {
				return err
			}
			if !lastUpdated.Equal(frozenLastUpdated) {
				return fmt.Errorf("PacemakerCluster lastUpdated advanced from %s to %s while status collector writes were blocked", frozenLastUpdated.Format(time.RFC3339), lastUpdated.Format(time.RFC3339))
			}
			return nil
		}, 90*time.Second, 10*time.Second).Should(o.Succeed(),
			"PacemakerCluster lastUpdated should remain frozen while status collector writes are blocked")

		staleDetectionDeadline := frozenLastUpdated.Add(7 * time.Minute)
		remainingDetectionTime := time.Until(staleDetectionDeadline)
		o.Expect(remainingDetectionTime).To(o.BeNumerically(">", 0), "stale detection budget should remain")

		g.By("Waiting for a fresh PacemakerStatusStale event")
		o.Expect(apis.WaitForPacemakerEvent(oc, apis.PacemakerHealthCheckEventNamespace, "PacemakerStatusStale", staleBaseline, remainingDetectionTime)).
			To(o.Succeed(), "expected PacemakerStatusStale after the status collector remains suspended")

		remainingDetectionTime = time.Until(staleDetectionDeadline)
		o.Expect(remainingDetectionTime).To(o.BeNumerically(">", 0), "stale detection budget should remain")

		g.By("Waiting for stale PacemakerHealthCheck degradation")
		o.Expect(apis.WaitForPacemakerHealthCheckDegraded(oc, "status is stale", remainingDetectionTime)).
			To(o.Succeed(), "PacemakerHealthCheckDegraded should report stale status")
		degradedObservedAt := time.Now()
		o.Expect(degradedObservedAt.Sub(frozenLastUpdated)).To(o.BeNumerically("<", 7*time.Minute),
			"PacemakerHealthCheckDegraded should be observed within seven minutes of frozen status")

		degraded, message, err := apis.IsPacemakerHealthCheckDegraded(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "expected to read PacemakerHealthCheckDegraded")
		o.Expect(degraded).To(o.BeTrue(), "PacemakerHealthCheckDegraded should be True for stale status")
		o.Expect(message).To(o.ContainSubstring("status is stale"), "degraded message should identify stale status")
		o.Expect(message).NotTo(o.ContainSubstring("is offline"), "stale status must not be reported as an offline node")

		g.By("Unblocking status collector writes")
		o.Expect(apis.UnblockStatusCollectorWrites(oc)).To(o.Succeed(), "expected to unblock status collector writes")

		g.By("Waiting for PacemakerCluster status to advance after recovery")
		o.Eventually(func() error {
			lastUpdated, err := apis.GetPacemakerClusterLastUpdated(oc)
			if err != nil {
				return err
			}
			if !lastUpdated.After(frozenLastUpdated) {
				return fmt.Errorf("PacemakerCluster lastUpdated has not advanced after unblocking status collector writes")
			}
			return nil
		}, 5*time.Minute, 10*time.Second).Should(o.Succeed(),
			"PacemakerCluster lastUpdated should advance after unblocking status collector writes")

		g.By("Waiting for PacemakerHealthCheckDegraded to clear")
		o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, 5*time.Minute)).
			To(o.Succeed(), "PacemakerHealthCheckDegraded should clear after unblocking status collector writes")

		// PacemakerHealthy is not emitted for stale-to-healthy recovery because CEO
		// keeps the last valid status as previous during Unknown, so
		// recordHealthTransitionEvents sees no transition. Recovery is asserted by
		// PacemakerHealthCheckDegraded clearing and lastUpdated advancing; re-add
		// this event assertion once the tracked OCPBUGS-127446 CEO bug is fixed.
	})

	g.It("should detect and recover from a node going offline via pcs cluster stop", func() {
		g.By("Verifying PacemakerCluster CR baseline is fully healthy before the test")
		o.Expect(apis.ExpectPacemakerBaseline(oc)).ToNot(o.HaveOccurred(), "expected PacemakerCluster to be fully healthy before test")
		waitForFreshPacemakerClusterStatus(oc)

		g.By("Capturing an event baseline before stopping Pacemaker")
		nodeOfflineBaseline := time.Now()

		g.By("Finding the collector-pinned node and its peer")
		pinnedNodeName, err := apis.GetStatusCollectorPinnedNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "expected to read status collector pinned node")
		o.Expect(pinnedNodeName).NotTo(o.BeEmpty(), "status collector CronJob must be pinned to a node")

		var collectorNode, collectorPeer corev1.Node
		collectorNodeFound := false
		collectorPeerFound := false
		for _, node := range nodes {
			if node.Name == pinnedNodeName {
				collectorNode = node
				collectorNodeFound = true
			} else {
				collectorPeer = node
				collectorPeerFound = true
			}
		}
		o.Expect(collectorNodeFound).To(o.BeTrue(), "collector-pinned node must be part of the two-node cluster")
		o.Expect(collectorPeerFound).To(o.BeTrue(), "collector-pinned node must have a peer")

		g.DeferCleanup(func() {
			framework.Logf("DeferCleanup: ensuring Pacemaker cluster is started")
			services.PcsClusterStartBestEffortViaDebug(oc, collectorPeer.Name, collectorNode.Name)
			waitForHealthCheckClearedBestEffort(oc)
		})

		g.By("Stopping Pacemaker on the collector-pinned node from its peer")
		err = services.PcsClusterStopViaDebug(oc, collectorPeer.Name, collectorNode.Name)
		o.Expect(err).To(o.BeNil(), "Expected pcs cluster stop to succeed")

		g.By("Waiting for collector rotation while checking status freshness every 30 seconds")
		o.Eventually(func() error {
			lastUpdated, err := apis.GetPacemakerClusterLastUpdated(oc)
			if err != nil {
				return err
			}
			if age := time.Since(lastUpdated); age >= 5*time.Minute {
				return fmt.Errorf("PacemakerCluster lastUpdated is %s old during collector rotation, expected < 5m", age.Round(time.Second))
			}

			currentPinnedNode, err := apis.GetStatusCollectorPinnedNode(oc)
			if err != nil {
				return err
			}
			if currentPinnedNode == collectorPeer.Name {
				return nil
			}
			return fmt.Errorf("status collector remains pinned to %s, expected rotation to %s", currentPinnedNode, collectorPeer.Name)
		}, 4*time.Minute, 30*time.Second).Should(o.Succeed(),
			"status collector should rotate from the stopped node to its peer while PacemakerCluster status stays fresh")
		o.Expect(apis.ExpectNoPacemakerEventSince(oc, apis.PacemakerHealthCheckEventNamespace, "PacemakerStatusStale", nodeOfflineBaseline)).To(o.Succeed(), "CR must not go stale during collector rotation")

		g.By("Waiting for PacemakerHealthCheckDegraded=True due to node offline")
		o.Expect(apis.WaitForPacemakerHealthCheckDegraded(oc, "is offline", apis.PacemakerDegradedDetectionTimeout)).
			ShouldNot(o.HaveOccurred(), "PacemakerHealthCheckDegraded should become True when a node is offline")

		degraded, message, err := apis.IsPacemakerHealthCheckDegraded(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "expected to read PacemakerHealthCheckDegraded")
		o.Expect(degraded).To(o.BeTrue(), "PacemakerHealthCheckDegraded should be True when the collector-pinned node is offline")
		o.Expect(message).To(o.ContainSubstring("is offline"), "degraded message should identify the offline path")
		o.Expect(message).NotTo(o.ContainSubstring("status is stale"), "degraded message should not identify the stale path")

		// TNF offline detection takes several minutes because etcd member loss triggers survivor re-bootstrap, so assert the event after the condition wait that carries the detection budget.
		g.By("Verifying PacemakerNodeOffline event was emitted")
		o.Expect(apis.WaitForPacemakerEvent(oc, apis.PacemakerHealthCheckEventNamespace, "PacemakerNodeOffline", nodeOfflineBaseline, 2*time.Minute)).
			To(o.Succeed(), "expected PacemakerNodeOffline event after stopping the collector-pinned node")

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

		g.By("Starting Pacemaker on the collector-pinned node")
		err = services.PcsClusterStartViaDebug(oc, collectorPeer.Name, collectorNode.Name)
		o.Expect(err).To(o.BeNil(), "Expected pcs cluster start to succeed")

		g.By("Waiting for PacemakerHealthCheckDegraded to clear")
		o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, healthCheckRecoveryTimeout)).
			ShouldNot(o.HaveOccurred(), "PacemakerHealthCheckDegraded should clear after node comes back online")

		g.By("Waiting for PacemakerCluster status to become fresh after node recovery")
		o.Eventually(func() error {
			lastUpdated, err := apis.GetPacemakerClusterLastUpdated(oc)
			if err != nil {
				return err
			}
			if age := time.Since(lastUpdated); age >= 3*time.Minute {
				return fmt.Errorf("PacemakerCluster lastUpdated is %s old after node recovery, expected < 3m", age.Round(time.Second))
			}
			return nil
		}, 5*time.Minute, 10*time.Second).Should(o.Succeed(),
			"PacemakerCluster lastUpdated should become fresh after node recovery")

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
