package two_node

import (
	"context"
	"fmt"
	"regexp"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/services"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	nodeutil "k8s.io/kubernetes/pkg/util/node"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	kubeletDisruptionTimeout     = 10 * time.Minute // Timeout for kubelet disruption scenarios
	kubeletRestoreTimeout        = 5 * time.Minute  // Time to wait for kubelet service restore
	kubeletGracePeriod           = 30 * time.Second // Grace period for kubelet to start/stop
	etcdStableDuringDisruption   = 5 * time.Minute  // Duration to assert etcd member stays healthy during disruption
	failureWindowClockSkewBuffer = 1 * time.Minute  // Buffer for clock skew when checking resource failure history
)

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Serial][Slow][Disruptive] Two Node with Fencing cluster", func() {
	defer g.GinkgoRecover()

	var (
		oc                = exutil.NewCLIWithoutNamespace("two-node-kubelet").AsAdmin()
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
		// Short-circuit if BeforeEach skipped before setup completed
		// (e.g., due to precondition failures like unhealthy cluster)
		if !setupCompleted {
			framework.Logf("Test was skipped before setup completed, skipping AfterEach cleanup")
			return
		}

		// Cleanup: Wait for both nodes to become healthy before performing cleanup operations.
		// If nodes don't recover, the test fails (as it should for a recovery test).
		g.By("Cleanup: Waiting for both nodes to become Ready")
		o.Eventually(func() error {
			nodeList, err := utils.GetNodes(oc, utils.AllNodes)
			if err != nil {
				return fmt.Errorf("failed to retrieve nodes: %v", err)
			}

			if len(nodeList.Items) != 2 {
				return fmt.Errorf("expected 2 nodes, found %d", len(nodeList.Items))
			}

			// Verify both nodes are Ready
			for _, node := range nodeList.Items {
				nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("failed to get node %s: %v", node.Name, err)
				}
				if !nodeutil.IsNodeReady(nodeObj) {
					return fmt.Errorf("node %s is not Ready", node.Name)
				}
			}

			framework.Logf("Both nodes are Ready")
			return nil
		}, kubeletRestoreTimeout, utils.FiveSecondPollInterval).Should(o.Succeed(), "Both nodes must be Ready before cleanup")

		// Both nodes are now healthy - perform cleanup operations
		nodeList, _ := utils.GetNodes(oc, utils.AllNodes)
		cleanupNode := nodeList.Items[1] // Use second node for cleanup commands

		g.By(fmt.Sprintf("Cleanup: Clearing any kubelet and etcd resource bans using node %s", cleanupNode.Name))
		for _, resource := range []string{"kubelet-clone", "etcd-clone"} {
			if cleanupErr := utils.RemoveConstraint(oc, cleanupNode.Name, resource); cleanupErr != nil {
				framework.Logf("Warning: Failed to clear %s: %v (expected if no bans were active)", resource, cleanupErr)
			} else {
				framework.Logf("Successfully cleared %s resource bans and failures", resource)
			}
		}

		g.By("Cleanup: Validating etcd cluster health")
		o.Eventually(func() error {
			return utils.LogEtcdClusterStatus(oc, "AfterEach cleanup", etcdClientFactory)
		}, kubeletRestoreTimeout, utils.FiveSecondPollInterval).Should(o.Succeed(), "Etcd cluster must be healthy after cleanup")
	})

	g.It("should recover from single node kubelet service disruption", func() {
		nodeList, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodeList.Items)).To(o.Equal(2), "Expected to find exactly 2 nodes for two-node cluster")

		nodes := nodeList.Items

		g.By("Ensuring both nodes are healthy before starting kubelet disruption test")
		for _, node := range nodes {
			if ready := nodeutil.IsNodeReady(&node); !ready {
				o.Expect(ready).Should(o.BeTrue(), fmt.Sprintf("Node %s should be ready before kubelet disruption", node.Name))
			}
		}

		targetNode := nodes[0]
		survivingNode := nodes[1]

		g.By(fmt.Sprintf("Banning kubelet resource from node: %s", targetNode.Name))
		err = utils.AddConstraint(oc, survivingNode.Name, "kubelet-clone", targetNode.Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to ban kubelet resource from node %s without errors", targetNode.Name))

		// Register cleanup to ensure ban is removed even if test fails
		g.DeferCleanup(func() {
			framework.Logf("DeferCleanup: Ensuring kubelet-clone ban is removed")
			cleanupErr := utils.RemoveConstraint(oc, survivingNode.Name, "kubelet-clone")
			if cleanupErr != nil {
				framework.Logf("DeferCleanup: Warning: Failed to clear kubelet-clone ban: %v (this is expected if already cleared)", cleanupErr)
			} else {
				framework.Logf("DeferCleanup: Successfully cleared kubelet-clone ban")
			}
		})

		g.By(fmt.Sprintf("Checking that node %s is not in state Ready due to kubelet resource ban", targetNode.Name))
		o.Eventually(func() bool {
			nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), targetNode.Name, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting node %s: %v", targetNode.Name, err)
				return false
			}
			return !nodeutil.IsNodeReady(nodeObj)
		}, kubeletDisruptionTimeout, utils.FiveSecondPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s is not in state Ready after kubelet resource ban is applied", targetNode.Name))

		g.By("Verifying PacemakerHealthCheckDegraded condition reports kubelet failure on target node")
		err = services.WaitForPacemakerHealthCheckDegraded(oc, "Kubelet", healthCheckDegradedTimeout, utils.FiveSecondPollInterval)
		o.Expect(err).NotTo(o.HaveOccurred(), "Pacemaker health check should report degraded due to kubelet constraint")
		// Assert degraded resource is Kubelet and that it is the node we banned (operator message format: "<node> node is unhealthy: Kubelet ...")
		o.Expect(services.AssertPacemakerHealthCheckContains(oc, []string{"Kubelet", targetNode.Name})).To(o.Succeed())

		g.By("Validating etcd cluster remains healthy with surviving node")
		o.Consistently(func() error {
			return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, survivingNode.Name)
		}, etcdStableDuringDisruption, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("etcd member %s should remain healthy during kubelet disruption", survivingNode.Name))

		g.By("Clearing kubelet resource bans to allow normal operation")
		err = utils.RemoveConstraint(oc, survivingNode.Name, "kubelet-clone")
		o.Expect(err).To(o.BeNil(), "Expected to clear kubelet resource bans without errors")

		g.By("Verifying PacemakerHealthCheckDegraded condition clears after recovery")
		o.Expect(services.WaitForPacemakerHealthCheckHealthy(oc, healthCheckHealthyTimeout, utils.FiveSecondPollInterval)).To(o.Succeed())

		g.By("Validating both nodes are Ready")
		for _, node := range nodes {
			o.Eventually(func() bool {
				nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return nodeutil.IsNodeReady(nodeObj)
			}, kubeletRestoreTimeout, utils.FiveSecondPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should be Ready", node.Name))
		}

		g.By("Validating etcd cluster fully recovered")
		o.Eventually(func() error {
			return utils.LogEtcdClusterStatus(oc, "after resource ban removal", etcdClientFactory)
		}, kubeletRestoreTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster should be healthy")

		g.By("Validating essential operators available")
		o.Eventually(func() error {
			return utils.ValidateEssentialOperatorsAvailable(oc)
		}, kubeletRestoreTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(), "Essential operators should be available")
	})

	g.It("should properly stop kubelet service and verify automatic restart on target node", func() {
		nodeList, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodeList.Items)).To(o.Equal(2), "Expected to find exactly 2 nodes for two-node cluster")

		nodes := nodeList.Items

		g.By("Ensuring both nodes are healthy before starting kubelet disruption test")
		for _, node := range nodes {
			o.Eventually(func() bool {
				nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{})
				if err != nil {
					framework.Logf("Error getting node %s: %v", node.Name, err)
					return false
				}
				return nodeutil.IsNodeReady(nodeObj)
			}, nodeIsHealthyTimeout, utils.FiveSecondPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should be ready before kubelet disruption", node.Name))
		}

		targetNode := nodes[0]
		survivingNode := nodes[1]

		g.By(fmt.Sprintf("Verifying kubelet service is initially running on target node: %s", targetNode.Name))
		o.Eventually(func() bool {
			isRunning := utils.IsServiceRunning(oc, survivingNode.Name, targetNode.Name, "kubelet")
			return isRunning
		}, kubeletGracePeriod, utils.FiveSecondPollInterval).Should(o.BeTrue(), fmt.Sprintf("Kubelet service should be running initially on node %s", targetNode.Name))

		// Record the time before stopping kubelet to filter failures
		stopTime := time.Now()

		g.By(fmt.Sprintf("Stopping kubelet service on target node: %s", targetNode.Name))
		err = utils.StopKubeletService(oc, targetNode.Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to stop kubelet service on node %s without errors", targetNode.Name))

		g.By("Waiting for Pacemaker to auto-recover and restart kubelet-clone service")
		o.Eventually(func() bool {
			isRunning := utils.IsServiceRunning(oc, survivingNode.Name, targetNode.Name, "kubelet")
			framework.Logf("Kubelet running on %s: %v", targetNode.Name, isRunning)
			return isRunning
		}, kubeletRestoreTimeout, utils.FiveSecondPollInterval).Should(o.BeTrue(), fmt.Sprintf("Kubelet should be running on %s after Pacemaker restart", targetNode.Name))

		g.By("Verifying Pacemaker recorded the kubelet failure in operation history")
		// Use a time window from when we stopped kubelet to now
		failureWindow := time.Since(stopTime) + failureWindowClockSkewBuffer
		hasFailure, failures, err := utils.HasRecentResourceFailure(oc, survivingNode.Name, "kubelet-clone", failureWindow)
		o.Expect(err).To(o.BeNil(), "Expected to check resource failure history without errors")
		o.Expect(hasFailure).To(o.BeTrue(), "Pacemaker should have recorded kubelet failure in operation history")
		framework.Logf("Pacemaker recorded %d failure(s) for kubelet-clone: %+v", len(failures), failures)

		g.By("Validating both nodes are Ready after Pacemaker restart")
		for _, node := range nodes {
			o.Eventually(func() bool {
				nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return nodeutil.IsNodeReady(nodeObj)
			}, kubeletRestoreTimeout, utils.FiveSecondPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should be Ready", node.Name))
		}

		g.By("Validating etcd cluster fully recovered")
		o.Eventually(func() error {
			return utils.LogEtcdClusterStatus(oc, "after kubelet restart", etcdClientFactory)
		}, kubeletRestoreTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster should be healthy")

		g.By("Validating essential operators available")
		o.Eventually(func() error {
			return utils.ValidateEssentialOperatorsAvailable(oc)
		}, kubeletRestoreTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(), "Essential operators should be available")
	})
})

// Etcd constraint / health check test lives in a separate Describe without [OCPFeatureGate:DualReplica];
// we do not add new tests under the FeatureGate-gated suite.
var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][Suite:openshift/two-node][Serial][Slow][Disruptive] Two Node etcd constraint and health check", func() {
	defer g.GinkgoRecover()

	var (
		oc                = exutil.NewCLIWithoutNamespace("two-node-etcd-constraint").AsAdmin()
		etcdClientFactory *helpers.EtcdClientFactoryImpl
	)

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)
		etcdClientFactory = helpers.NewEtcdClientFactory(oc.KubeClient())
		utils.SkipIfClusterIsNotHealthy(oc, etcdClientFactory)
	})

	g.It("should recover from etcd resource location constraint with health check degraded then healthy", func() {
		nodeList, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodeList.Items)).To(o.Equal(2), "Expected to find exactly 2 nodes for two-node cluster")
		nodes := nodeList.Items
		targetNode := nodes[0]
		survivingNode := nodes[1]

		g.By("Ensuring both nodes are healthy before applying etcd constraint")
		for _, node := range nodes {
			o.Expect(nodeutil.IsNodeReady(&node)).To(o.BeTrue(), fmt.Sprintf("Node %s should be ready", node.Name))
		}

		g.By(fmt.Sprintf("Banning etcd resource from node %s (location constraint)", targetNode.Name))
		err = utils.AddConstraint(oc, survivingNode.Name, "etcd-clone", targetNode.Name)
		o.Expect(err).To(o.BeNil(), "Expected to ban etcd-clone from target node")
		g.DeferCleanup(func() {
			_ = utils.RemoveConstraint(oc, survivingNode.Name, "etcd-clone")
		})

		g.By("Verifying PacemakerHealthCheckDegraded condition reports etcd failure on target node")
		// Operator message format: "<nodeName> node is unhealthy: Etcd has failed" (or "is stopped", etc.)
		degradedPattern := regexp.QuoteMeta(targetNode.Name) + ` node is unhealthy: Etcd .*`
		err = services.WaitForPacemakerHealthCheckDegraded(oc, degradedPattern, healthCheckDegradedTimeout, utils.FiveSecondPollInterval)
		o.Expect(err).NotTo(o.HaveOccurred(), "Pacemaker health check should report degraded due to etcd constraint")
		o.Expect(services.AssertPacemakerHealthCheckContains(oc, []string{"Etcd", targetNode.Name})).To(o.Succeed())

		g.By("Removing etcd-clone constraint to restore normal operation")
		err = utils.RemoveConstraint(oc, survivingNode.Name, "etcd-clone")
		o.Expect(err).To(o.BeNil(), "Expected to clear etcd-clone constraint")

		g.By("Verifying PacemakerHealthCheckDegraded condition clears after recovery")
		o.Expect(services.WaitForPacemakerHealthCheckHealthy(oc, healthCheckHealthyTimeout, utils.FiveSecondPollInterval)).To(o.Succeed())

		g.By("Validating etcd cluster is healthy")
		o.Eventually(func() error {
			return utils.LogEtcdClusterStatus(oc, "after etcd constraint removal", etcdClientFactory)
		}, kubeletRestoreTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred())
	})
})
