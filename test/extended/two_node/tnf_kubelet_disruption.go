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
	corev1 "k8s.io/api/core/v1"
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
		nodes             []corev1.Node
	)

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)

		g.By("Verifying comprehensive etcd cluster status before starting kubelet disruption test")
		o.Eventually(func() error {
			return utils.LogEtcdClusterStatus(oc, "BeforeEach validation")
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster should be fully healthy before starting test")

		nodeList, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodeList.Items)).To(o.Equal(2), "Expected to find exactly 2 nodes for two-node cluster")

		nodes = nodeList.Items
		framework.Logf("Found nodes: %s and %s for kubelet disruption test", nodes[0].Name, nodes[1].Name)

		kubeClient := oc.KubeClient()
		etcdClientFactory = helpers.NewEtcdClientFactory(kubeClient)

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

		g.By("Validating essential operators are available before kubelet disruption")
		o.Eventually(func() error {
			return utils.ValidateEssentialOperatorsAvailable(oc)
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "Essential cluster operators should be available before kubelet disruption")
	})

	g.AfterEach(func() {
		// Cleanup: Remove any resource bans that may have been created during the test
		// This ensures the device under test is in the same state the test started in
		if len(nodes) == 2 {
			survivingNode := nodes[1] // Use the second node as the surviving node for cleanup commands

			g.By("Cleanup: Clearing any kubelet resource bans that may exist")
			framework.Logf("Cleanup: Clearing all bans and failures for kubelet-clone resource")
			cleanupErr := utils.RemoveConstraint(oc, survivingNode.Name, "kubelet-clone")
			if cleanupErr != nil {
				framework.Logf("Warning: Failed to clear kubelet-clone resource during cleanup: %v (this is expected if no bans were active)", cleanupErr)
			} else {
				framework.Logf("Successfully cleared all bans and failures for kubelet-clone resource during cleanup")
			}

			g.By("Cleanup: Waiting for all nodes to become Ready after resource ban cleanup")
			for _, node := range nodes {
				o.Eventually(func() bool {
					nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{})
					if err != nil {
						framework.Logf("Error getting node %s: %v", node.Name, err)
						return false
					}
					return nodeutil.IsNodeReady(nodeObj)
				}, kubeletRestoreTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should be Ready after cleanup", node.Name))
			}

			g.By("Cleanup: Validating etcd cluster status after test cleanup")
			o.Eventually(func() error {
				return utils.LogEtcdClusterStatus(oc, "AfterEach cleanup")
			}, kubeletRestoreTimeout, kubeletPollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster should be healthy after test cleanup")
		}
	})

	g.It("Should recover from single node kubelet service disruption", func() {
		targetNode := nodes[0]
		survivingNode := nodes[1]

		g.By(fmt.Sprintf("Banning kubelet resource from node: %s", targetNode.Name))
		err := utils.AddConstraint(oc, survivingNode.Name, "kubelet-clone", targetNode.Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to ban kubelet resource from node %s without errors", targetNode.Name))

		g.By("Checking that the node is not in state Ready due to kubelet resource ban")
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
			return utils.LogEtcdClusterStatus(oc, "resource ban removal recovery")
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

	g.It("Should properly stop kubelet service and verify automatic restart on target node", func() {
		targetNode := nodes[0]
		survivingNode := nodes[1]

		framework.Logf("Starting kubelet service disruption test: target node=%s, surviving node=%s",
			targetNode.Name, survivingNode.Name)

		g.By(fmt.Sprintf("Verifying kubelet service is initially running on target node: %s", targetNode.Name))
		framework.Logf("Checking initial kubelet service status on target node %s", targetNode.Name)
		o.Eventually(func() bool {
			return utils.IsServiceRunning(oc, targetNode.Name, "kubelet")
		}, kubeletGracePeriod, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Kubelet service should be running initially on node %s", targetNode.Name))
		framework.Logf("Confirmed kubelet service is running initially on target node %s", targetNode.Name)

		g.By(fmt.Sprintf("Stopping kubelet service on target node: %s", targetNode.Name))
		framework.Logf("Attempting to stop kubelet service on target node %s", targetNode.Name)
		err := utils.StopKubeletService(oc, targetNode.Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to stop kubelet service on node %s without errors", targetNode.Name))
		framework.Logf("Successfully stopped kubelet service on target node %s", targetNode.Name)

		g.By("Validating etcd cluster eventually becomes healthy with surviving node during kubelet disruption")
		framework.Logf("Starting etcd health validation on surviving node %s (timeout: %v)", survivingNode.Name, kubeletDisruptionTimeout)
		o.Eventually(func() error {
			return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, survivingNode.Name)
		}, kubeletDisruptionTimeout, pollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("etcd member %s should remain healthy during kubelet service disruption", survivingNode.Name))
		framework.Logf("Confirmed etcd member %s remains healthy during kubelet disruption", survivingNode.Name)

		g.By("Waiting for kubelet service to automatically restart on target node")
		framework.Logf("Monitoring kubelet service for automatic restart on target node %s (timeout: %v)", targetNode.Name, kubeletRestoreTimeout)
		o.Eventually(func() bool {
			return utils.IsServiceRunning(oc, targetNode.Name, "kubelet")
		}, kubeletRestoreTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Kubelet service should automatically restart on node %s", targetNode.Name))
		framework.Logf("Kubelet service successfully restarted automatically on target node %s", targetNode.Name)

		g.By("Validating both nodes are Ready after kubelet service automatic restart")
		framework.Logf("Starting node readiness validation after kubelet restart")
		for _, node := range nodes {
			framework.Logf("Checking readiness of node %s", node.Name)
			o.Eventually(func() bool {
				nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{})
				if err != nil {
					framework.Logf("Error getting node %s: %v", node.Name, err)
					return false
				}
				return nodeutil.IsNodeReady(nodeObj)
			}, kubeletRestoreTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should be Ready after kubelet automatic restart", node.Name))
			framework.Logf("Node %s is Ready after kubelet restart", node.Name)
		}
		framework.Logf("All nodes are Ready after kubelet service restart")

		g.By("Ensuring both etcd members are healthy after kubelet service automatic restart")
		framework.Logf("Starting etcd member health validation after kubelet restart")
		for _, node := range nodes {
			framework.Logf("Validating etcd member health on node %s", node.Name)
			o.Eventually(func() error {
				return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, node.Name)
			}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("etcd member %s should be healthy after kubelet automatic restart", node.Name))
			framework.Logf("Etcd member on node %s is healthy after kubelet restart", node.Name)
		}
		framework.Logf("All etcd members are healthy after kubelet service restart")

		g.By("Validating comprehensive etcd cluster recovery after kubelet service automatic restart")
		framework.Logf("Starting comprehensive etcd cluster validation after kubelet restart")
		o.Eventually(func() error {
			return utils.LogEtcdClusterStatus(oc, "kubelet service restart recovery")
		}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster should be fully healthy after kubelet automatic restart")
		framework.Logf("Comprehensive etcd cluster validation completed successfully after kubelet restart")

		g.By("Validating essential operators recovery after kubelet service automatic restart")
		framework.Logf("Starting essential operators availability validation after kubelet restart")
		o.Eventually(func() error {
			return utils.ValidateEssentialOperatorsAvailable(oc)
		}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "Essential cluster operators should be available after kubelet automatic restart")
		framework.Logf("All cluster operators are available after kubelet restart")

		framework.Logf("Kubelet service disruption test completed successfully - full recovery validated")
	})

})
