package two_node

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	nodeutil "k8s.io/kubernetes/pkg/util/node"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	kubeletDisruptionTimeout = 10 * time.Minute // Timeout for kubelet disruption scenarios
	kubeletRestoreTimeout    = 5 * time.Minute  // Time to wait for kubelet service restore
	kubeletPollInterval      = 10 * time.Second // Poll interval for kubelet status checks
	kubeletGracePeriod       = 30 * time.Second // Grace period for kubelet to start/stop
)

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Serial][Slow][Disruptive] Two Node with Fencing cluster", func() {
	defer g.GinkgoRecover()

	var (
		oc                = util.NewCLIWithoutNamespace("").SetNamespace("openshift-etcd").AsAdmin()
		etcdClientFactory *helpers.EtcdClientFactoryImpl
	)

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)

		kubeClient := oc.KubeClient()
		etcdClientFactory = helpers.NewEtcdClientFactory(kubeClient)

		g.By("Verifying comprehensive etcd cluster status before starting kubelet disruption test")
		o.Eventually(func() error {
			return utils.LogEtcdClusterStatus(oc, "BeforeEach validation", etcdClientFactory)
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster should be fully healthy before starting test")

		g.By("Validating essential operators are available before kubelet disruption")
		o.Eventually(func() error {
			return utils.ValidateEssentialOperatorsAvailable(oc)
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "Essential cluster operators should be available before kubelet disruption")

		framework.Logf("BeforeEach completed successfully - cluster is ready for kubelet disruption test")
	})

	g.AfterEach(func() {
		// Cleanup: Remove any resource bans that may have been created during the test
		// This ensures the device under test is in the same state the test started in
		nodeList, err := utils.GetNodes(oc, utils.AllNodes)
		if err != nil {
			framework.Logf("Critical: Failed to retrieve nodes for cleanup - test isolation cannot be guaranteed: %v", err)
			return
		}

		// Track if node count is unexpected, but attempt cleanup anyway
		nodeCountUnexpected := len(nodeList.Items) != 2
		if nodeCountUnexpected {
			framework.Logf("Warning: Expected 2 nodes but found %d - attempting cleanup anyway", len(nodeList.Items))
		}

		// Attempt cleanup if we have at least 1 node
		if len(nodeList.Items) >= 1 {
			// Use the last available node for cleanup commands (prefer second node if available)
			cleanupNode := nodeList.Items[0]
			if len(nodeList.Items) >= 2 {
				cleanupNode = nodeList.Items[1]
			}

			g.By(fmt.Sprintf("Cleanup: Clearing any kubelet resource bans that may exist using node %s", cleanupNode.Name))
			cleanupErr := utils.RemoveConstraint(oc, cleanupNode.Name, "kubelet-clone")
			if cleanupErr != nil {
				framework.Logf("Warning: Failed to clear kubelet-clone resource during cleanup: %v (this is expected if no bans were active)", cleanupErr)
			} else {
				framework.Logf("Successfully cleared all bans and failures for kubelet-clone resource during cleanup")
			}

			g.By("Cleanup: Waiting for all nodes to become Ready after resource ban cleanup")
			for _, node := range nodeList.Items {
				// Use a non-blocking check with logging instead of assertion
				ready := false
				for i := 0; i < int(kubeletRestoreTimeout/kubeletPollInterval); i++ {
					nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{})
					if err == nil && nodeutil.IsNodeReady(nodeObj) {
						ready = true
						framework.Logf("Node %s is Ready after cleanup", node.Name)
						break
					}
					time.Sleep(kubeletPollInterval)
				}
				if !ready {
					framework.Logf("Warning: Node %s did not become Ready within timeout after cleanup", node.Name)
				}
			}

			g.By("Cleanup: Validating etcd cluster status after test cleanup")
			// Use a non-blocking check with logging instead of assertion
			etcdHealthy := false
			for i := 0; i < int(kubeletRestoreTimeout/kubeletPollInterval); i++ {
				if err := utils.LogEtcdClusterStatus(oc, "AfterEach cleanup", etcdClientFactory); err == nil {
					etcdHealthy = true
					framework.Logf("Etcd cluster is healthy after cleanup")
					break
				}
				time.Sleep(kubeletPollInterval)
			}
			if !etcdHealthy {
				framework.Logf("Warning: Etcd cluster did not become healthy within timeout after cleanup")
			}
		} else {
			framework.Logf("Critical: Cannot perform cleanup - no nodes available")
		}

		// Log node count issue but don't fail - cleanup should always complete
		if nodeCountUnexpected {
			framework.Logf("Warning: Expected exactly 2 nodes for two-node cluster but found %d - cluster topology may have changed during test", len(nodeList.Items))
		}
	})

	g.It("should recover from single node kubelet service disruption", func() {
		nodeList, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodeList.Items)).To(o.Equal(2), "Expected to find exactly 2 nodes for two-node cluster")

		nodes := nodeList.Items
		framework.Logf("Found nodes: %s and %s for kubelet disruption test", nodes[0].Name, nodes[1].Name)

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
		}, kubeletDisruptionTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s is not in state Ready after kubelet resource ban is applied", targetNode.Name))

		g.By(fmt.Sprintf("Ensuring surviving node %s remains Ready during kubelet disruption", survivingNode.Name))
		o.Consistently(func() bool {
			nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), survivingNode.Name, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting node %s: %v", survivingNode.Name, err)
				return false
			}
			return nodeutil.IsNodeReady(nodeObj)
		}, 2*time.Minute, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Surviving node %s should remain Ready during kubelet disruption", survivingNode.Name))

		g.By("Validating etcd cluster remains healthy with surviving node")
		o.Consistently(func() error {
			return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, survivingNode.Name)
		}, kubeletDisruptionTimeout, pollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("etcd member %s should remain healthy during kubelet disruption", survivingNode.Name))

		g.By("Clearing kubelet resource bans to allow normal operation")
		err = utils.RemoveConstraint(oc, survivingNode.Name, "kubelet-clone")
		o.Expect(err).To(o.BeNil(), "Expected to clear kubelet resource bans without errors")

		g.By("Waiting for target node to become Ready after kubelet resource unban")
		o.Eventually(func() bool {
			nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), targetNode.Name, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting node %s: %v", targetNode.Name, err)
				return false
			}
			return nodeutil.IsNodeReady(nodeObj)
		}, kubeletRestoreTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should become Ready after kubelet resource ban removal", targetNode.Name))

		g.By("Validating both nodes are Ready after kubelet resource ban removal")
		for _, node := range nodes {
			o.Eventually(func() bool {
				nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{})
				if err != nil {
					framework.Logf("Error getting node %s: %v", node.Name, err)
					return false
				}
				return nodeutil.IsNodeReady(nodeObj)
			}, kubeletRestoreTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should be Ready after kubelet resource ban removal", node.Name))
		}

		g.By("Validating comprehensive etcd cluster recovery after kubelet resource ban removal")
		o.Eventually(func() error {
			return utils.LogEtcdClusterStatus(oc, "resource ban removal recovery", etcdClientFactory)
		}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster should be fully healthy after kubelet resource ban removal")

		g.By("Ensuring both etcd members are healthy after kubelet resource ban removal")
		for _, node := range nodes {
			o.Eventually(func() error {
				return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, node.Name)
			}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("etcd member %s should be healthy after kubelet resource ban removal", node.Name))
		}

		g.By("Validating essential operators recovery after kubelet resource ban disruption")
		o.Eventually(func() error {
			return utils.ValidateEssentialOperatorsAvailable(oc)
		}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "Essential cluster operators should be available after kubelet resource ban removal")
	})

	g.It("should properly stop kubelet service and verify automatic restart on target node", func() {
		nodeList, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodeList.Items)).To(o.Equal(2), "Expected to find exactly 2 nodes for two-node cluster")

		nodes := nodeList.Items
		framework.Logf("Found nodes: %s and %s for kubelet disruption test", nodes[0].Name, nodes[1].Name)

		g.By("Ensuring both nodes are healthy before starting kubelet disruption test")
		for _, node := range nodes {
			o.Eventually(func() bool {
				nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{})
				if err != nil {
					framework.Logf("Error getting node %s: %v", node.Name, err)
					return false
				}
				return nodeutil.IsNodeReady(nodeObj)
			}, nodeIsHealthyTimeout, pollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should be ready before kubelet disruption", node.Name))
		}

		targetNode := nodes[0]
		survivingNode := nodes[1]

		framework.Logf("DEBUG: Starting kubelet service test - Target node: %s, Surviving node: %s", targetNode.Name, survivingNode.Name)

		g.By(fmt.Sprintf("Verifying kubelet service is initially running on target node: %s", targetNode.Name))
		o.Eventually(func() bool {
			isRunning := utils.IsServiceRunning(oc, survivingNode.Name, targetNode.Name, "kubelet")
			framework.Logf("DEBUG: Initial kubelet service status check - node: %s, running: %v", targetNode.Name, isRunning)
			return isRunning
		}, kubeletGracePeriod, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Kubelet service should be running initially on node %s", targetNode.Name))

		framework.Logf("DEBUG: About to stop kubelet service on target node: %s", targetNode.Name)
		g.By(fmt.Sprintf("Stopping kubelet service on target node: %s", targetNode.Name))
		err = utils.StopKubeletService(oc, targetNode.Name)
		framework.Logf("DEBUG: StopKubeletService result - node: %s, error: %v", targetNode.Name, err)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to stop kubelet service on node %s without errors", targetNode.Name))

		g.By("Validating etcd cluster eventually becomes healthy with surviving node during kubelet disruption")
		framework.Logf("DEBUG: Starting etcd health validation with surviving node: %s", survivingNode.Name)
		o.Eventually(func() error {
			err := helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, survivingNode.Name)
			framework.Logf("DEBUG: etcd health check - surviving node: %s, error: %v", survivingNode.Name, err)
			return err
		}, kubeletDisruptionTimeout, pollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("etcd member %s should remain healthy during kubelet service disruption", survivingNode.Name))

		g.By("Waiting for kubelet service to automatically restart on target node")
		framework.Logf("DEBUG: Starting wait for kubelet service automatic restart - target node: %s, timeout: %v", targetNode.Name, kubeletRestoreTimeout)
		o.Eventually(func() bool {
			isRunning := utils.IsServiceRunning(oc, survivingNode.Name, targetNode.Name, "kubelet")
			framework.Logf("DEBUG: Kubelet automatic restart check - target node: %s, running: %v", targetNode.Name, isRunning)
			return isRunning
		}, kubeletRestoreTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Kubelet service should automatically restart on node %s", targetNode.Name))

		g.By("Validating both nodes are Ready after kubelet service automatic restart")
		framework.Logf("DEBUG: Starting node readiness validation after kubelet restart - checking %d nodes", len(nodes))
		for _, node := range nodes {
			framework.Logf("DEBUG: Checking node readiness for node: %s", node.Name)
			o.Eventually(func() bool {
				nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{})
				if err != nil {
					framework.Logf("DEBUG: Error getting node %s: %v", node.Name, err)
					return false
				}
				isReady := nodeutil.IsNodeReady(nodeObj)
				framework.Logf("DEBUG: Node readiness check - node: %s, ready: %v", node.Name, isReady)
				return isReady
			}, kubeletRestoreTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should be Ready after kubelet automatic restart", node.Name))
		}

		g.By("Ensuring both etcd members are healthy after kubelet service automatic restart")
		framework.Logf("DEBUG: Starting etcd member health validation after kubelet restart - checking %d members", len(nodes))
		for _, node := range nodes {
			framework.Logf("DEBUG: Checking etcd member health for node: %s", node.Name)
			o.Eventually(func() error {
				err := helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, node.Name)
				framework.Logf("DEBUG: etcd member health check - node: %s, error: %v", node.Name, err)
				return err
			}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("etcd member %s should be healthy after kubelet automatic restart", node.Name))
		}

		g.By("Validating comprehensive etcd cluster recovery after kubelet service automatic restart")
		framework.Logf("DEBUG: Starting comprehensive etcd cluster status validation after kubelet restart")
		o.Eventually(func() error {
			err := utils.LogEtcdClusterStatus(oc, "kubelet service restart recovery", etcdClientFactory)
			framework.Logf("DEBUG: Comprehensive etcd cluster status check - error: %v", err)
			return err
		}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster should be fully healthy after kubelet automatic restart")

		g.By("Validating essential operators recovery after kubelet service automatic restart")
		framework.Logf("DEBUG: Starting essential operators validation after kubelet restart")
		o.Eventually(func() error {
			err := utils.ValidateEssentialOperatorsAvailable(oc)
			framework.Logf("DEBUG: Essential operators validation - error: %v", err)
			return err
		}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "Essential cluster operators should be available after kubelet automatic restart")

		framework.Logf("DEBUG: Kubelet service test completed successfully - all validations passed")
	})

})
