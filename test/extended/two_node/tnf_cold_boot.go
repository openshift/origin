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
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	coldBootTimeout        = 30 * time.Minute // Extended timeout for cold boot recovery
	coldBootPollInterval   = 30 * time.Second // Longer poll interval for cold boot
	nodeRestartDelay       = 2 * time.Minute  // Wait time between node reboots
	clusterRecoveryTimeout = 20 * time.Minute // Time to wait for cluster recovery
)

var _ = g.Describe("[sig-node][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Disruptive] Two Node Cold Boot Recovery", func() {
	defer g.GinkgoRecover()

	var (
		oc                = util.NewCLIWithoutNamespace("").AsAdmin()
		etcdClientFactory *helpers.EtcdClientFactoryImpl
		nodes             []corev1.Node
	)

	g.BeforeEach(func() {
		skipIfNotTopology(oc, v1.DualReplicaTopologyMode)

		g.By("Verifying etcd cluster operator is healthy before starting cold boot test")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy before starting test")

		nodeList, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodeList.Items)).To(o.Equal(2), "Expected to find exactly 2 nodes for two-node cluster")

		nodes = nodeList.Items
		g.GinkgoT().Printf("Found nodes: %s and %s for cold boot test\n", nodes[0].Name, nodes[1].Name)

		kubeClient := oc.KubeClient()
		etcdClientFactory = helpers.NewEtcdClientFactory(kubeClient)

		g.By("Ensuring both nodes are healthy before starting cold boot test")
		for _, node := range nodes {
			o.Eventually(func() error {
				return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, node.Name)
			}, nodeIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("expect node %s to be healthy before cold boot", node.Name))
		}

		g.By("Validating cluster is in good state before cold boot")
		o.Expect(validateClusterOperatorsAvailable(oc)).To(o.Succeed(), "All cluster operators should be available before cold boot")
	})

	g.It("Should recover from complete cluster graceful shutdown and restart", func() {
		g.By("Performing graceful shutdown: shutting down both nodes simultaneously")

		// Shut down both nodes simultaneously
		g.By(fmt.Sprintf("Gracefully shutting down first node: %s", nodes[0].Name))
		err := util.TriggerNodeShutdownGraceful(oc.KubeClient(), nodes[0].Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to gracefully shutdown node %s without errors", nodes[0].Name))

		g.By(fmt.Sprintf("Gracefully shutting down second node: %s", nodes[1].Name))
		err = util.TriggerNodeShutdownGraceful(oc.KubeClient(), nodes[1].Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to gracefully shutdown node %s without errors", nodes[1].Name))

		g.By("Waiting for both nodes to complete shutdown and external restart cycle")
		time.Sleep(5 * time.Minute) // Allow time for both nodes to shutdown and be restarted externally

		g.By("Waiting for both nodes to become ready after graceful shutdown and restart")
		for _, node := range nodes {
			o.Eventually(func() bool {
				return isNodeReady(oc, node.Name)
			}, coldBootTimeout, coldBootPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should become ready after graceful shutdown and restart", node.Name))
		}

		g.By("Validating etcd cluster recovery after graceful shutdown and restart")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, clusterRecoveryTimeout, coldBootPollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy after graceful shutdown and restart")

		g.By("Ensuring both etcd members are healthy and voting after graceful shutdown and restart")
		for _, node := range nodes {
			o.Eventually(func() error {
				return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, node.Name)
			}, clusterRecoveryTimeout, coldBootPollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("etcd member %s should be healthy after graceful shutdown and restart", node.Name))
		}

		g.By("Validating cluster operators recovery after graceful shutdown and restart")
		o.Eventually(func() error {
			return validateClusterOperatorsAvailable(oc)
		}, clusterRecoveryTimeout, coldBootPollInterval).ShouldNot(o.HaveOccurred(), "All cluster operators should be available after graceful shutdown and restart")

		g.By("Validating critical system pods are running after graceful shutdown and restart")
		o.Eventually(func() error {
			return validateCriticalPodsRunning(oc)
		}, clusterRecoveryTimeout, coldBootPollInterval).ShouldNot(o.HaveOccurred(), "All critical system pods should be running after graceful shutdown and restart")

		g.By("Validating etcd cluster has proper membership after graceful shutdown and restart")
		validateEtcdMembershipAfterColdBoot(etcdClientFactory, nodes)
	})

	g.It("Should recover from ungraceful complete cluster shutdown and restart", func() {
		g.By("Performing ungraceful shutdown: force shutting down both nodes simultaneously without clean termination")

		g.By(fmt.Sprintf("Force shutting down first node: %s", nodes[0].Name))
		err := util.TriggerNodeShutdownUngraceful(oc.KubeClient(), nodes[0].Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to ungracefully shutdown node %s without errors", nodes[0].Name))

		g.By(fmt.Sprintf("Force shutting down second node: %s", nodes[1].Name))
		err = util.TriggerNodeShutdownUngraceful(oc.KubeClient(), nodes[1].Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to ungracefully shutdown node %s without errors", nodes[1].Name))

		g.By("Waiting for both nodes to complete ungraceful shutdown and external restart cycle")
		time.Sleep(5 * time.Minute) // Allow time for both nodes to shutdown and be restarted externally

		g.By("Waiting for both nodes to become ready after ungraceful shutdown and restart")
		for _, node := range nodes {
			o.Eventually(func() bool {
				return isNodeReady(oc, node.Name)
			}, coldBootTimeout, coldBootPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should become ready after ungraceful shutdown and restart", node.Name))
		}

		g.By("Validating etcd cluster recovery after ungraceful shutdown and restart")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, clusterRecoveryTimeout, coldBootPollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy after ungraceful shutdown and restart")

		g.By("Ensuring both etcd members are healthy after ungraceful shutdown and restart")
		for _, node := range nodes {
			o.Eventually(func() error {
				return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, node.Name)
			}, clusterRecoveryTimeout, coldBootPollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("etcd member %s should be healthy after ungraceful shutdown and restart", node.Name))
		}

		g.By("Validating cluster operators recovery after ungraceful shutdown and restart")
		o.Eventually(func() error {
			return validateClusterOperatorsAvailable(oc)
		}, clusterRecoveryTimeout, coldBootPollInterval).ShouldNot(o.HaveOccurred(), "All cluster operators should be available after ungraceful shutdown and restart")

		g.By("Validating critical system pods are running after ungraceful shutdown and restart")
		o.Eventually(func() error {
			return validateCriticalPodsRunning(oc)
		}, clusterRecoveryTimeout, coldBootPollInterval).ShouldNot(o.HaveOccurred(), "All critical system pods should be running after ungraceful shutdown and restart")

		g.By("Validating etcd cluster has proper membership after ungraceful shutdown and restart")
		validateEtcdMembershipAfterColdBoot(etcdClientFactory, nodes)
	})
})

// isNodeReady checks if a node is in Ready state
func isNodeReady(oc *util.CLI, nodeName string) bool {
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		framework.Logf("Error getting node %s: %v", nodeName, err)
		return false
	}

	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// validateClusterOperatorsAvailable ensures all cluster operators are available
func validateClusterOperatorsAvailable(oc *util.CLI) error {
	clusterOperators, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list cluster operators: %v", err)
	}

	for _, operator := range clusterOperators.Items {
		if !isClusterOperatorAvailable(&operator) {
			return fmt.Errorf("cluster operator %s is not available", operator.Name)
		}
		if isClusterOperatorDegraded(&operator) {
			return fmt.Errorf("cluster operator %s is degraded", operator.Name)
		}
	}

	framework.Logf("All %d cluster operators are available and not degraded", len(clusterOperators.Items))
	return nil
}

// validateCriticalPodsRunning ensures critical system pods are running
func validateCriticalPodsRunning(oc *util.CLI) error {
	criticalNamespaces := []string{
		"openshift-etcd",
		"openshift-kube-apiserver",
		"openshift-kube-controller-manager",
		"openshift-kube-scheduler",
		"openshift-machine-config-operator",
	}

	for _, namespace := range criticalNamespaces {
		pods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list pods in namespace %s: %v", namespace, err)
		}

		runningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning && util.CheckPodIsReady(pod) {
				runningPods++
			}
		}

		if runningPods == 0 {
			return fmt.Errorf("no running pods found in critical namespace %s", namespace)
		}
		framework.Logf("Found %d running pods in namespace %s", runningPods, namespace)
	}

	return nil
}

// validateEtcdMembershipAfterColdBoot validates etcd cluster membership after cold boot
func validateEtcdMembershipAfterColdBoot(etcdClientFactory *helpers.EtcdClientFactoryImpl, nodes []corev1.Node) {
	g.By("Validating etcd cluster membership after cold boot")

	o.Eventually(func() error {
		members, err := getMembers(etcdClientFactory)
		if err != nil {
			return fmt.Errorf("failed to get etcd members: %v", err)
		}

		if len(members) != 2 {
			return fmt.Errorf("expected 2 etcd members, found %d", len(members))
		}

		// Ensure both nodes are started and voting members (not learners)
		for _, node := range nodes {
			started, learner, err := getMemberState(&node, members)
			if err != nil {
				return fmt.Errorf("failed to get member state for node %s: %v", node.Name, err)
			}
			if !started {
				return fmt.Errorf("node %s is not started in etcd cluster", node.Name)
			}
			if learner {
				return fmt.Errorf("node %s is still a learner, should be voting member after cold boot", node.Name)
			}
		}

		framework.Logf("etcd cluster membership is healthy after cold boot: 2 voting members")
		return nil
	}, clusterRecoveryTimeout, coldBootPollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster membership should be healthy after cold boot")
}
