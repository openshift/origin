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
	"github.com/openshift/origin/test/extended/etcd/helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	healthCheckRecoveryTimeout = 10 * time.Minute
)

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

		nodeList, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).To(o.BeNil(), "Expected to retrieve nodes without error")
		o.Expect(len(nodeList.Items)).To(o.Equal(2), "Expected exactly 2 nodes for two-node cluster")

		randomIndex := rand.Intn(len(nodeList.Items))
		execNode = nodeList.Items[randomIndex]
		targetNode = nodeList.Items[(randomIndex+1)%len(nodeList.Items)]
		nodes = nodeList.Items

		g.DeferCleanup(func() {
			logFinalClusterStatus(nodes)
		})
	})

	g.It("should detect and recover from cluster maintenance mode", func() {
		g.By("Enabling cluster maintenance mode")
		_, err := exutil.DebugNodeRetryWithOptionsAndChroot(
			oc, execNode.Name, "default", "bash", "-c",
			"sudo pcs property set maintenance-mode=true")
		o.Expect(err).To(o.BeNil(), "Expected to enable maintenance mode")

		g.DeferCleanup(func() {
			framework.Logf("DeferCleanup: Ensuring maintenance mode is disabled")
			if _, cleanupErr := exutil.DebugNodeRetryWithOptionsAndChroot(
				oc, execNode.Name, "default", "bash", "-c",
				"sudo pcs property set maintenance-mode=false 2>/dev/null; true"); cleanupErr != nil {
				framework.Logf("Warning: Failed to disable maintenance mode: %v", cleanupErr)
			}
		})

		g.By("Waiting for PacemakerHealthCheckDegraded=True due to maintenance mode")
		o.Expect(apis.WaitForPacemakerHealthCheckDegraded(oc, "maintenance mode", healthCheckRecoveryTimeout)).
			ShouldNot(o.HaveOccurred(), "PacemakerHealthCheckDegraded should become True when cluster is in maintenance mode")

		g.By("Verifying PacemakerClusterInMaintenance event was emitted")
		o.Expect(apis.WaitForPacemakerEvent(oc, "PacemakerClusterInMaintenance", 2*time.Minute)).
			ShouldNot(o.HaveOccurred(), "Expected PacemakerClusterInMaintenance event in openshift-etcd namespace")

		g.By("Disabling cluster maintenance mode")
		_, err = exutil.DebugNodeRetryWithOptionsAndChroot(
			oc, execNode.Name, "default", "bash", "-c",
			"sudo pcs property set maintenance-mode=false")
		o.Expect(err).To(o.BeNil(), "Expected to disable maintenance mode")

		g.By("Waiting for PacemakerHealthCheckDegraded to clear")
		o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, healthCheckRecoveryTimeout)).
			ShouldNot(o.HaveOccurred(), "PacemakerHealthCheckDegraded should clear after maintenance mode is disabled")

		g.By("Verifying PacemakerHealthy event was emitted after recovery")
		o.Expect(apis.WaitForPacemakerEvent(oc, "PacemakerHealthy", 2*time.Minute)).
			ShouldNot(o.HaveOccurred(), "Expected PacemakerHealthy event in openshift-etcd namespace after recovery")

		g.By("Validating cluster health after maintenance mode recovery")
		o.Eventually(func() error {
			return utils.ValidateEssentialOperatorsAvailable(oc)
		}, healthCheckRecoveryTimeout, utils.FiveSecondPollInterval).ShouldNot(
			o.HaveOccurred(), "Essential operators should be available after maintenance mode recovery")
	})

	g.It("should detect and recover from a node going offline via pcs cluster stop", func() {
		g.By(fmt.Sprintf("Stopping pacemaker cluster on %s from peer node %s", targetNode.Name, execNode.Name))
		_, err := exutil.DebugNodeRetryWithOptionsAndChroot(
			oc, execNode.Name, "default", "bash", "-c",
			fmt.Sprintf("sudo pcs cluster stop %s", targetNode.Name))
		o.Expect(err).To(o.BeNil(), "Expected pcs cluster stop to succeed")

		g.DeferCleanup(func() {
			framework.Logf("DeferCleanup: Ensuring pacemaker cluster is started on %s", targetNode.Name)
			if _, cleanupErr := exutil.DebugNodeRetryWithOptionsAndChroot(
				oc, execNode.Name, "default", "bash", "-c",
				fmt.Sprintf("sudo pcs cluster start %s 2>/dev/null; true", targetNode.Name)); cleanupErr != nil {
				framework.Logf("Warning: Failed to restart pacemaker on %s: %v", targetNode.Name, cleanupErr)
			}
		})

		g.By("Waiting for PacemakerHealthCheckDegraded=True due to node offline")
		o.Expect(apis.WaitForPacemakerHealthCheckDegraded(oc, "is offline", healthCheckRecoveryTimeout)).
			ShouldNot(o.HaveOccurred(), "PacemakerHealthCheckDegraded should become True when a node is offline")

		g.By("Checking PacemakerCluster CR NodeCountAsExpected condition while node is offline")
		pc, pcErr := apis.GetPacemakerCluster(oc)
		if pcErr != nil {
			framework.Logf("Warning: could not read PacemakerCluster CR: %v", pcErr)
		} else if ncErr := apis.ExpectClusterNodeCountAsExpected(pc); ncErr != nil {
			framework.Logf("NodeCountAsExpected is False while node is offline (expected): %v", ncErr)
		} else {
			framework.Logf("NodeCountAsExpected remains True while node is offline (pcs cluster stop does not affect corosync node count)")
		}

		g.By(fmt.Sprintf("Starting pacemaker cluster on %s", targetNode.Name))
		_, err = exutil.DebugNodeRetryWithOptionsAndChroot(
			oc, execNode.Name, "default", "bash", "-c",
			fmt.Sprintf("sudo pcs cluster start %s", targetNode.Name))
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
	})
})
