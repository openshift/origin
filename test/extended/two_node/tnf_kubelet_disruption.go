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
	kubeletDisruptionTimeout = 10 * time.Minute // Timeout for kubelet disruption scenarios
	kubeletRestoreTimeout    = 5 * time.Minute  // Time to wait for kubelet service restore
	kubeletPollInterval      = 10 * time.Second // Poll interval for kubelet status checks
	kubeletGracePeriod       = 30 * time.Second // Grace period for kubelet to start/stop
)

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Serial][Slow][Disruptive] Two Node Kubelet Service Disruption", func() {
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
				return isNodeReady(oc, node.Name)
			}, nodeIsHealthyTimeout, pollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should be ready before kubelet disruption", node.Name))
		}

		g.By("Validating cluster is in good state before kubelet disruption")
		o.Eventually(func() error {
			return validateClusterOperatorsAvailable(oc)
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "All cluster operators should be available before kubelet disruption")
	})

	g.It("Should recover from single node kubelet service disruption", func() {
		targetNode := nodes[0]
		survivingNode := nodes[1]

		g.By(fmt.Sprintf("Stopping kubelet service on node: %s", targetNode.Name))
		err := stopKubeletService(oc, targetNode.Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to stop kubelet service on node %s without errors", targetNode.Name))

		g.By("Waiting for target node to become NotReady due to kubelet service stop")
		o.Eventually(func() bool {
			return !isNodeReady(oc, targetNode.Name)
		}, kubeletDisruptionTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should become NotReady after kubelet service stop", targetNode.Name))

		g.By(fmt.Sprintf("Ensuring surviving node %s remains Ready during kubelet disruption", survivingNode.Name))
		o.Consistently(func() bool {
			return isNodeReady(oc, survivingNode.Name)
		}, 2*time.Minute, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Surviving node %s should remain Ready during kubelet disruption", survivingNode.Name))

		g.By("Validating etcd cluster remains healthy with surviving node")
		o.Eventually(func() error {
			return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, survivingNode.Name)
		}, kubeletDisruptionTimeout, pollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("etcd member %s should remain healthy during kubelet disruption", survivingNode.Name))

		g.By(fmt.Sprintf("Restoring kubelet service on node: %s", targetNode.Name))
		err = startKubeletService(oc, targetNode.Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to start kubelet service on node %s without errors", targetNode.Name))

		g.By("Waiting for target node to become Ready after kubelet service restore")
		o.Eventually(func() bool {
			return isNodeReady(oc, targetNode.Name)
		}, kubeletRestoreTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should become Ready after kubelet service restore", targetNode.Name))

		g.By("Validating both nodes are Ready after kubelet service restore")
		for _, node := range nodes {
			o.Eventually(func() bool {
				return isNodeReady(oc, node.Name)
			}, kubeletRestoreTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should be Ready after kubelet service restore", node.Name))
		}

		g.By("Validating etcd cluster recovery after kubelet service restore")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy after kubelet service restore")

		g.By("Ensuring both etcd members are healthy after kubelet service restore")
		for _, node := range nodes {
			o.Eventually(func() error {
				return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, node.Name)
			}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("etcd member %s should be healthy after kubelet service restore", node.Name))
		}

		g.By("Validating cluster operators recovery after kubelet service disruption")
		o.Eventually(func() error {
			return validateClusterOperatorsAvailable(oc)
		}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "All cluster operators should be available after kubelet service restore")
	})

	g.It("Should recover from sequential kubelet service disruption on both nodes", func() {
		firstNode := nodes[0]
		secondNode := nodes[1]

		g.By(fmt.Sprintf("Stopping kubelet service on first node: %s", firstNode.Name))
		err := stopKubeletService(oc, firstNode.Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to stop kubelet service on node %s without errors", firstNode.Name))

		g.By("Waiting for first node to become NotReady")
		o.Eventually(func() bool {
			return !isNodeReady(oc, firstNode.Name)
		}, kubeletDisruptionTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should become NotReady after kubelet service stop", firstNode.Name))

		g.By("Allowing time for cluster to adapt to first node kubelet disruption")
		time.Sleep(kubeletGracePeriod)

		g.By(fmt.Sprintf("Restoring kubelet service on first node: %s", firstNode.Name))
		err = startKubeletService(oc, firstNode.Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to start kubelet service on node %s without errors", firstNode.Name))

		g.By("Waiting for first node to become Ready before disrupting second node")
		o.Eventually(func() bool {
			return isNodeReady(oc, firstNode.Name)
		}, kubeletRestoreTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should become Ready after kubelet service restore", firstNode.Name))

		g.By("Allowing time for cluster to stabilize before second disruption")
		time.Sleep(kubeletGracePeriod)

		g.By(fmt.Sprintf("Stopping kubelet service on second node: %s", secondNode.Name))
		err = stopKubeletService(oc, secondNode.Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to stop kubelet service on node %s without errors", secondNode.Name))

		g.By("Waiting for second node to become NotReady")
		o.Eventually(func() bool {
			return !isNodeReady(oc, secondNode.Name)
		}, kubeletDisruptionTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should become NotReady after kubelet service stop", secondNode.Name))

		g.By(fmt.Sprintf("Ensuring first node %s remains Ready during second node kubelet disruption", firstNode.Name))
		o.Consistently(func() bool {
			return isNodeReady(oc, firstNode.Name)
		}, 1*time.Minute, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("First node %s should remain Ready during second node kubelet disruption", firstNode.Name))

		g.By(fmt.Sprintf("Restoring kubelet service on second node: %s", secondNode.Name))
		err = startKubeletService(oc, secondNode.Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to start kubelet service on node %s without errors", secondNode.Name))

		g.By("Waiting for both nodes to become Ready after sequential kubelet disruption")
		for _, node := range nodes {
			o.Eventually(func() bool {
				return isNodeReady(oc, node.Name)
			}, kubeletRestoreTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should become Ready after sequential kubelet disruption", node.Name))
		}

		g.By("Validating etcd cluster recovery after sequential kubelet disruption")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy after sequential kubelet disruption")

		g.By("Ensuring both etcd members are healthy after sequential kubelet disruption")
		for _, node := range nodes {
			o.Eventually(func() error {
				return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, node.Name)
			}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("etcd member %s should be healthy after sequential kubelet disruption", node.Name))
		}

		g.By("Validating cluster operators recovery after sequential kubelet disruption")
		o.Eventually(func() error {
			return validateClusterOperatorsAvailable(oc)
		}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "All cluster operators should be available after sequential kubelet disruption")
	})

	g.It("Should handle kubelet service disruption with workload validation", func() {
		targetNode := nodes[0]

		g.By("Creating test workload before kubelet disruption")
		testNamespace := "kubelet-disruption-test"
		err := createTestWorkload(oc, testNamespace)
		o.Expect(err).To(o.BeNil(), "Expected to create test workload without errors")
		defer cleanupTestWorkload(oc, testNamespace)

		g.By("Validating test workload is running before disruption")
		o.Eventually(func() bool {
			return isTestWorkloadHealthy(oc, testNamespace)
		}, 2*time.Minute, kubeletPollInterval).Should(o.BeTrue(), "Test workload should be healthy before kubelet disruption")

		g.By(fmt.Sprintf("Stopping kubelet service on node: %s", targetNode.Name))
		err = stopKubeletService(oc, targetNode.Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to stop kubelet service on node %s without errors", targetNode.Name))

		g.By("Waiting for target node to become NotReady due to kubelet service stop")
		o.Eventually(func() bool {
			return !isNodeReady(oc, targetNode.Name)
		}, kubeletDisruptionTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should become NotReady after kubelet service stop", targetNode.Name))

		g.By("Validating test workload resilience during kubelet disruption")
		// Workload may temporarily show issues but should eventually recover on surviving node
		time.Sleep(kubeletGracePeriod) // Allow time for pod eviction/rescheduling

		g.By(fmt.Sprintf("Restoring kubelet service on node: %s", targetNode.Name))
		err = startKubeletService(oc, targetNode.Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to start kubelet service on node %s without errors", targetNode.Name))

		g.By("Waiting for target node to become Ready after kubelet service restore")
		o.Eventually(func() bool {
			return isNodeReady(oc, targetNode.Name)
		}, kubeletRestoreTimeout, kubeletPollInterval).Should(o.BeTrue(), fmt.Sprintf("Node %s should become Ready after kubelet service restore", targetNode.Name))

		g.By("Validating test workload recovery after kubelet service restore")
		o.Eventually(func() bool {
			return isTestWorkloadHealthy(oc, testNamespace)
		}, 3*time.Minute, kubeletPollInterval).Should(o.BeTrue(), "Test workload should recover after kubelet service restore")

		g.By("Validating cluster full recovery after kubelet disruption with workload")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, kubeletRestoreTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy after kubelet disruption with workload")
	})
})

// stopKubeletService stops the kubelet service on the specified node
func stopKubeletService(oc *util.CLI, nodeName string) error {
	framework.Logf("Stopping kubelet service on node %s", nodeName)

	cmd := []string{
		"chroot", "/host",
		"systemctl", "stop", "kubelet",
	}

	_, err := util.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	if err != nil {
		return fmt.Errorf("failed to stop kubelet service on node %s: %v", nodeName, err)
	}

	framework.Logf("Successfully stopped kubelet service on node %s", nodeName)
	return nil
}

// startKubeletService starts the kubelet service on the specified node
func startKubeletService(oc *util.CLI, nodeName string) error {
	framework.Logf("Starting kubelet service on node %s", nodeName)

	cmd := []string{
		"chroot", "/host",
		"systemctl", "start", "kubelet",
	}

	_, err := util.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	if err != nil {
		return fmt.Errorf("failed to start kubelet service on node %s: %v", nodeName, err)
	}

	framework.Logf("Successfully started kubelet service on node %s", nodeName)
	return nil
}

// isKubeletServiceRunning checks if kubelet service is running on the specified node
func isKubeletServiceRunning(oc *util.CLI, nodeName string) bool {
	cmd := []string{
		"chroot", "/host",
		"systemctl", "is-active", "--quiet", "kubelet",
	}

	_, err := util.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	return err == nil
}

// createTestWorkload creates a simple test deployment for workload validation
func createTestWorkload(oc *util.CLI, namespace string) error {
	framework.Logf("Creating test workload in namespace %s", namespace)

	// Create namespace
	_, err := oc.AsAdmin().Run("create").Args("namespace", namespace).Output()
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %v", namespace, err)
	}

	// Create test deployment
	deploymentYAML := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubelet-disruption-test
  namespace: %s
spec:
  replicas: 2
  selector:
    matchLabels:
      app: kubelet-disruption-test
  template:
    metadata:
      labels:
        app: kubelet-disruption-test
    spec:
      containers:
      - name: test-container
        image: registry.redhat.io/ubi8/ubi-minimal:latest
        command: ['sleep', 'infinity']
        resources:
          requests:
            memory: "64Mi"
            cpu: "50m"
          limits:
            memory: "128Mi"
            cpu: "100m"
      tolerations:
      - operator: Exists
`, namespace)

	err = oc.AsAdmin().Run("apply").Args("-f", "-").InputString(deploymentYAML).Execute()
	if err != nil {
		return fmt.Errorf("failed to create test deployment: %v", err)
	}

	return nil
}

// isTestWorkloadHealthy checks if the test workload is healthy
func isTestWorkloadHealthy(oc *util.CLI, namespace string) bool {
	output, err := oc.AsAdmin().Run("get").Args("deployment", "kubelet-disruption-test", "-n", namespace, "-o", "jsonpath={.status.readyReplicas}").Output()
	if err != nil {
		framework.Logf("Error checking test workload health: %v", err)
		return false
	}

	if output == "2" {
		return true
	}

	framework.Logf("Test workload not fully ready: %s/2 replicas ready", output)
	return false
}

// cleanupTestWorkload removes the test workload
func cleanupTestWorkload(oc *util.CLI, namespace string) {
	framework.Logf("Cleaning up test workload in namespace %s", namespace)
	_ = oc.AsAdmin().Run("delete").Args("namespace", namespace, "--ignore-not-found=true").Execute()
}
