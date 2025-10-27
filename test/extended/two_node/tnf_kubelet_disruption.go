package two_node

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/etcd/helpers"
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
		oc                = util.NewCLIWithoutNamespace("").AsAdmin()
		etcdClientFactory *helpers.EtcdClientFactoryImpl
		nodes             []corev1.Node
	)

	g.BeforeEach(func() {
		skipIfNotTopology(oc, v1.DualReplicaTopologyMode)

		g.By("Verifying etcd cluster operator is healthy before starting kubelet disruption test")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy before starting test")

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

		g.By("Validating cluster is in good state before kubelet disruption")
		o.Eventually(func() error {
			return validateClusterOperatorsAvailable(oc)
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "All cluster operators should be available before kubelet disruption")
	})

	g.AfterEach(func() {
		// Cleanup: Remove any constraints that may have been created during the test
		// This ensures the device under test is in the same state the test started in
		if len(nodes) == 2 {
			survivingNode := nodes[1] // Use the second node as the surviving node for cleanup commands

			g.By("Cleanup: Attempting to remove any kubelet constraints that may exist")
			for _, targetNode := range nodes {
				constraintId, err := discoverConstraintId(oc, survivingNode.Name, "kubelet-clone", targetNode.Name)
				if err != nil {
					framework.Logf("No constraint found for kubelet-clone resource avoiding node %s (this is expected if no constraint was created)", targetNode.Name)
					continue
				}

				if constraintId != "" {
					framework.Logf("Cleanup: Found constraint ID %s for kubelet-clone avoiding node %s, removing it", constraintId, targetNode.Name)
					cleanupErr := removeConstraint(oc, survivingNode.Name, constraintId)
					if cleanupErr != nil {
						framework.Logf("Warning: Failed to remove constraint %s during cleanup: %v", constraintId, cleanupErr)
					} else {
						framework.Logf("Successfully removed constraint %s during cleanup", constraintId)
					}
				}
			}

			g.By("Cleanup: Waiting for all nodes to become Ready after constraint cleanup")
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
		}
	})

	g.It("Should recover from single node kubelet service disruption", func() {
		targetNode := nodes[0]
		survivingNode := nodes[1]
		var constraintId string

		g.By(fmt.Sprintf("Adding constraint to avoid kubelet resource on node: %s", targetNode.Name))
		err := addConstraint(oc, survivingNode.Name, "kubelet-clone", targetNode.Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to add constraint for kubelet resource on node %s without errors", targetNode.Name))

		g.By("Discovering the constraint ID for later removal")
		constraintId, err = discoverConstraintId(oc, survivingNode.Name, "kubelet-clone", targetNode.Name)
		o.Expect(err).To(o.BeNil(), "Expected to discover constraint ID without errors")
		framework.Logf("Discovered constraint ID: %s", constraintId)

		g.By("Checking that the node is not in state Ready due to kubelet constraint")
		o.Eventually(func() bool {
			nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), targetNode.Name, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting node %s: %v", targetNode.Name, err)
				return false
			}
			return !nodeutil.IsNodeReady(nodeObj)
		}, kubeletDisruptionTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s is not in state Ready after kubelet constraint is applied", targetNode.Name))

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

		g.By(fmt.Sprintf("Removing constraint (ID: %s) to allow kubelet resource on node: %s", constraintId, targetNode.Name))
		err = removeConstraint(oc, survivingNode.Name, constraintId)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to remove constraint %s for kubelet resource on node %s without errors", constraintId, targetNode.Name))

		g.By("Waiting for target node to become Ready after kubelet constraint removal")
		o.Eventually(func() bool {
			nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), targetNode.Name, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting node %s: %v", targetNode.Name, err)
				return false
			}
			return nodeutil.IsNodeReady(nodeObj)
		}, kubeletRestoreTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should become Ready after kubelet constraint removal", targetNode.Name))

		g.By("Validating both nodes are Ready after kubelet constraint removal")
		for _, node := range nodes {
			o.Eventually(func() bool {
				nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{})
				if err != nil {
					framework.Logf("Error getting node %s: %v", node.Name, err)
					return false
				}
				return nodeutil.IsNodeReady(nodeObj)
			}, kubeletRestoreTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should be Ready after kubelet constraint removal", node.Name))
		}

		g.By("Validating etcd cluster recovery after kubelet constraint removal")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy after kubelet constraint removal")

		g.By("Ensuring both etcd members are healthy after kubelet constraint removal")
		for _, node := range nodes {
			o.Eventually(func() error {
				return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, node.Name)
			}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("etcd member %s should be healthy after kubelet constraint removal", node.Name))
		}

		g.By("Validating cluster operators recovery after kubelet constraint disruption")
		o.Eventually(func() error {
			return validateClusterOperatorsAvailable(oc)
		}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "All cluster operators should be available after kubelet constraint removal")
	})

	g.It("Should properly stop kubelet service and verify automatic restart on target node", func() {
		targetNode := nodes[0]
		survivingNode := nodes[1]

		framework.Logf("Starting kubelet service disruption test: target node=%s, surviving node=%s",
			targetNode.Name, survivingNode.Name)

		g.By(fmt.Sprintf("Verifying kubelet service is initially running on target node: %s", targetNode.Name))
		framework.Logf("Checking initial kubelet service status on target node %s", targetNode.Name)
		o.Eventually(func() bool {
			return isServiceRunning(oc, targetNode.Name, "kubelet")
		}, kubeletGracePeriod, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Kubelet service should be running initially on node %s", targetNode.Name))
		framework.Logf("Confirmed kubelet service is running initially on target node %s", targetNode.Name)

		g.By(fmt.Sprintf("Stopping kubelet service on target node: %s", targetNode.Name))
		framework.Logf("Attempting to stop kubelet service on target node %s", targetNode.Name)
		err := stopKubeletService(oc, targetNode.Name)
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
			return isServiceRunning(oc, targetNode.Name, "kubelet")
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

		g.By("Validating etcd cluster recovery after kubelet service automatic restart")
		framework.Logf("Starting etcd cluster operator health validation after kubelet restart")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy after kubelet automatic restart")
		framework.Logf("Etcd cluster operator is healthy after kubelet restart")

		g.By("Validating cluster operators recovery after kubelet service automatic restart")
		framework.Logf("Starting cluster operators availability validation after kubelet restart")
		o.Eventually(func() error {
			return validateClusterOperatorsAvailable(oc)
		}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "All cluster operators should be available after kubelet automatic restart")
		framework.Logf("All cluster operators are available after kubelet restart")

		framework.Logf("Kubelet service disruption test completed successfully - full recovery validated")
	})

})
