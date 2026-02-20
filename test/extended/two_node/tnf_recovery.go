package two_node

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
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
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	nodeIsHealthyTimeout            = time.Minute
	etcdOperatorIsHealthyTimeout    = time.Minute
	memberHasLeftTimeout            = 5 * time.Minute
	memberIsLeaderTimeout           = 20 * time.Minute
	memberRejoinedLearnerTimeout    = 10 * time.Minute
	memberPromotedVotingTimeout     = 15 * time.Minute
	networkDisruptionDuration = 15 * time.Second
	vmRestartTimeout                = 5 * time.Minute
	vmUngracefulShutdownTimeout     = 30 * time.Second // Ungraceful VM shutdown is typically fast
	vmGracefulShutdownTimeout       = 10 * time.Minute // Graceful VM shutdown is typically slow
	membersHealthyAfterDoubleReboot = 30 * time.Minute // Includes full VM reboot and etcd member healthy
	progressLogInterval             = time.Minute      // Target interval for progress logging

	fencingJobName        = "tnf-fencing-job"
	fencingJobNamespace   = "openshift-etcd"
	fencingJobWaitTimeout = 10 * time.Minute
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

		// Graceful shutdown does not fence the node; skip fencing-event assertion and proceed to member-list validation.
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

		g.By("Verifying PacemakerHealthCheckDegraded condition clears after recovery")
		o.Expect(services.WaitForPacemakerHealthCheckHealthy(oc, healthCheckHealthyTimeoutAfterFencing, utils.FiveSecondPollInterval)).To(o.Succeed())
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

		g.By("Verifying that a fencing event was recorded for the node")
		o.Expect(services.WaitForFencingEvent(oc, []string{targetNode.Name}, healthCheckDegradedTimeoutAfterFencing, utils.FiveSecondPollInterval)).To(o.Succeed())

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

		g.By("Verifying PacemakerHealthCheckDegraded condition clears after recovery")
		o.Expect(services.WaitForPacemakerHealthCheckHealthy(oc, healthCheckHealthyTimeoutAfterFencing, utils.FiveSecondPollInterval)).To(o.Succeed())
	})

	g.It("should recover from network disruption with etcd member re-addition", func() {
		// Note: In network disruption, the targetNode runs the disruption command that
		// isolates the nodes from each other, creating a split-brain where pacemaker
		// determines which node gets fenced and which becomes the etcd leader.
		g.GinkgoT().Printf("Randomly selected %s (%s) to run the network disruption command\n", targetNode.Name, targetNode.Status.Addresses[0].Address)
		g.By(fmt.Sprintf("Blocking network communication between %s and %s for %v ", targetNode.Name, peerNode.Name, networkDisruptionDuration))
		command, err := exutil.TriggerNetworkDisruption(oc.KubeClient(), &targetNode, &peerNode, networkDisruptionDuration)
		o.Expect(err).To(o.BeNil(), "Expected to disrupt network without errors")
		g.GinkgoT().Printf("command: '%s'\n", command)

		g.By("Verifying that a fencing event was recorded for one of the nodes")
		o.Expect(services.WaitForFencingEvent(oc, []string{targetNode.Name, peerNode.Name}, healthCheckDegradedTimeoutAfterFencing, utils.FiveSecondPollInterval)).To(o.Succeed())

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

		g.By("Verifying PacemakerHealthCheckDegraded condition clears after recovery")
		o.Expect(services.WaitForPacemakerHealthCheckHealthy(oc, healthCheckHealthyTimeoutAfterFencing, utils.FiveSecondPollInterval)).To(o.Succeed())
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

		// Both nodes failed at once; they may have the same etcd revision and restart together without fencing.
		g.By("Restarting both nodes")
		restartVms(dataPair, c)

		g.By(fmt.Sprintf("Waiting both etcd members to become healthy (timeout: %v)", membersHealthyAfterDoubleReboot))
		// Both nodes are expected to be healthy voting members. The order of nodes passed to the validation function does not matter.
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&nodeA,
			&nodeB, true, false,
			membersHealthyAfterDoubleReboot, utils.FiveSecondPollInterval)

		g.By("Verifying PacemakerHealthCheckDegraded condition clears after recovery")
		o.Expect(services.WaitForPacemakerHealthCheckHealthy(oc, healthCheckHealthyTimeoutAfterFencing, utils.FiveSecondPollInterval)).To(o.Succeed())
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

		g.By("Verifying PacemakerHealthCheckDegraded condition clears after recovery")
		o.Expect(services.WaitForPacemakerHealthCheckHealthy(oc, healthCheckHealthyTimeoutAfterFencing, utils.FiveSecondPollInterval)).To(o.Succeed())
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

		// No survivor to fence the last node; first node left gracefully and will rejoin as learner.
		g.By("Restarting both nodes")
		restartVms(dataPair, c)

		g.By(fmt.Sprintf("Waiting both etcd members to become healthy (timeout: %v)", membersHealthyAfterDoubleReboot))
		// Both nodes are expected to be healthy voting members. The order of nodes passed to the validation function does not matter.
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&firstToShutdown,
			&secondToShutdown, true, false,
			membersHealthyAfterDoubleReboot, utils.FiveSecondPollInterval)

		g.By("Verifying PacemakerHealthCheckDegraded condition clears after recovery")
		o.Expect(services.WaitForPacemakerHealthCheckHealthy(oc, healthCheckHealthyTimeoutAfterFencing, utils.FiveSecondPollInterval)).To(o.Succeed())
	})

	g.It("should recover from BMC credential rotation with fencing", func() {
		secretName := "fencing-credentials-" + targetNode.Name
		backupDir, err := os.MkdirTemp("", "tnf-fencing-backup-")
		o.Expect(err).To(o.BeNil(), "Expected to create backup directory")
		defer func() {
			_ = os.RemoveAll(backupDir)
		}()

		// Backup the fencing secret (copy from OpenShift to disk) so we can restore it after
		// corrupting Pacemaker; delete+recreate with the same content triggers the fencing job
		// to push the correct credentials back into Pacemaker.
		g.By("Backing up fencing secret")
		o.Expect(core.BackupResource(oc, "secret", secretName, fencingJobNamespace, backupDir)).To(o.Succeed())

		g.By("Updating pacemaker stonith with wrong password to simulate fencing degraded")
		stonithID := targetNode.Name + "_redfish"
		o.Expect(utils.PcsStonithUpdatePassword(oc, peerNode.Name, stonithID, "wrongpassword")).To(o.Succeed())

		// Ensure we always restore the secret on exit so the cluster is not left with wrong Pacemaker credentials.
		backupFile := filepath.Join(backupDir, secretName+".yaml")
		defer func() {
			_, _ = oc.AsAdmin().Run("delete").Args("secret", secretName, "-n", fencingJobNamespace, "--ignore-not-found").Output()
			if err := core.RestoreResource(oc, backupFile); err != nil {
				framework.Logf("Warning: deferred restore of fencing secret failed: %v", err)
			}
		}()

		g.By("Verifying PacemakerHealthCheckDegraded condition reports fencing unavailable for target node")
		o.Expect(services.WaitForPacemakerHealthCheckDegraded(oc, `fencing unavailable \(no agents running\)`, healthCheckDegradedTimeoutAfterFencing, utils.FiveSecondPollInterval)).To(o.Succeed())
		o.Expect(services.AssertPacemakerHealthCheckContains(oc, []string{targetNode.Name, "fencing unavailable"})).To(o.Succeed())

		g.By("Restoring fencing secret (delete and recreate to trigger fencing job)")
		_, _ = oc.AsAdmin().Run("delete").Args("secret", secretName, "-n", fencingJobNamespace, "--ignore-not-found").Output()
		o.Expect(core.RestoreResource(oc, backupFile)).To(o.Succeed())

		g.By("Waiting for fencing job to complete so Pacemaker gets the correct credentials")
		kubeClient := oc.AdminKubeClient()
		o.Eventually(func() error {
			job, err := kubeClient.BatchV1().Jobs(fencingJobNamespace).Get(context.Background(), fencingJobName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if job.Status.CompletionTime == nil {
				return fmt.Errorf("job %s not yet complete", fencingJobName)
			}
			return nil
		}, fencingJobWaitTimeout, utils.FiveSecondPollInterval).Should(o.Succeed(), "fencing job must complete before asserting healthy")

		g.By("Verifying PacemakerHealthCheckDegraded condition clears")
		o.Expect(services.WaitForPacemakerHealthCheckHealthy(oc, healthCheckHealthyTimeoutAfterFencing, utils.FiveSecondPollInterval)).To(o.Succeed())
	})
})

// Fencing secret rotation test lives in a separate Describe without [OCPFeatureGate:DualReplica];
// we do not add new tests under the FeatureGate-gated suite.
var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][Suite:openshift/two-node][Serial][Disruptive] Two Node fencing secret rotation", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = exutil.NewCLIWithoutNamespace("tnf-fencing-secret-rotation").AsAdmin()
		etcdClientFactory    *helpers.EtcdClientFactoryImpl
		peerNode, targetNode corev1.Node
	)

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)
		etcdClientFactory = helpers.NewEtcdClientFactory(oc.KubeClient())
		utils.SkipIfClusterIsNotHealthy(oc, etcdClientFactory)

		nodes, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		randomIndex := rand.Intn(len(nodes.Items))
		peerNode = nodes.Items[randomIndex]
		targetNode = nodes.Items[(randomIndex+1)%len(nodes.Items)]
	})

	g.It("should reject invalid fencing credential updates and keep PacemakerCluster healthy", func() {
		kubeClient := oc.AdminKubeClient()

		ns, secretName, originalPassword, err := apis.RotateNodeBMCPassword(kubeClient, &targetNode)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to rotate BMC credentials without error")

		defer func() {
			if err := apis.RestoreBMCPassword(kubeClient, ns, secretName, originalPassword); err != nil {
				fmt.Fprintf(g.GinkgoWriter,
					"Warning: failed to restore original BMC password in %s/%s: %v\n",
					ns, secretName, err)
			}
		}()
		g.By("Ensuring etcd members remain healthy after BMC credential rotation")
		o.Eventually(func() error {
			if err := helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, peerNode.Name); err != nil {
				return err
			}
			if err := helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, targetNode.Name); err != nil {
				return err
			}
			return nil
		}, nodeIsHealthyTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(), "etcd members should be healthy after BMC credential rotation")

		g.By("Triggering fencing job by patching fencing secret so we can assert it refuses to update pacemaker")
		fencingSecretName := "fencing-credentials-" + targetNode.Name
		secret, err := kubeClient.CoreV1().Secrets(fencingJobNamespace).Get(context.Background(), fencingSecretName, metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred(), "get fencing secret for node %s", targetNode.Name)
		patched := secret.DeepCopy()
		if patched.Data == nil {
			patched.Data = make(map[string][]byte)
		}
		patched.Data["test-trigger"] = []byte("1")
		_, err = kubeClient.CoreV1().Secrets(fencingJobNamespace).Update(context.Background(), patched, metav1.UpdateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred(), "patch fencing secret to trigger job")

		g.By("Waiting for fencing job to complete")
		o.Eventually(func() error {
			job, err := kubeClient.BatchV1().Jobs(fencingJobNamespace).Get(context.Background(), fencingJobName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if job.Status.CompletionTime == nil {
				return fmt.Errorf("job %s not yet complete", fencingJobName)
			}
			return nil
		}, fencingJobWaitTimeout, utils.FiveSecondPollInterval).Should(o.Succeed(), "fencing job must complete")

		g.By("Verifying the secret was never rotated in Pacemaker: fencing job logs show it refused to update stonith")
		pods, err := kubeClient.CoreV1().Pods(fencingJobNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: "job-name=" + fencingJobName})
		o.Expect(err).ToNot(o.HaveOccurred(), "list pods for fencing job")
		o.Expect(pods.Items).ToNot(o.BeEmpty(), "fencing job should have at least one pod (job may not have been recreated after secret patch)")
		pod := &pods.Items[0]
		podName := pod.Name
		framework.Logf("Fencing job pod: %s (namespace: %s) phase=%s", podName, fencingJobNamespace, pod.Status.Phase)
		for i, cs := range pod.Status.ContainerStatuses {
			framework.Logf("  container[%d] %s: ready=%v state=%+v", i, cs.Name, cs.Ready, cs.State)
		}
		// CRI-O may return "unable to retrieve container logs" briefly after the job completes; retry a few times.
		const logRetries = 3
		const logRetryDelay = 5 * time.Second
		var podLogs string
		for attempt := 0; attempt < logRetries; attempt++ {
			podLogs, err = oc.AsAdmin().Run("logs").Args(podName, "-n", fencingJobNamespace).Output()
			if err != nil {
				framework.Logf("Get fencing job pod logs (attempt %d/%d): %v", attempt+1, logRetries, err)
				if attempt < logRetries-1 {
					time.Sleep(logRetryDelay)
				}
				continue
			}
			if strings.Contains(podLogs, "unable to retrieve container logs") && attempt < logRetries-1 {
				framework.Logf("Pod logs contained 'unable to retrieve container logs' (attempt %d/%d), retrying in %v", attempt+1, logRetries, logRetryDelay)
				time.Sleep(logRetryDelay)
				continue
			}
			break
		}
		o.Expect(err).ToNot(o.HaveOccurred(), "get fencing job pod logs after %d attempts", logRetries)
		framework.Logf("Fencing job pod: %s (namespace: %s). Full pod logs:\n%s", podName, fencingJobNamespace, podLogs)
		if strings.Contains(podLogs, "unable to retrieve container logs") {
			framework.Failf("could not retrieve container logs for fencing job pod %s (CRI-O may have purged them); pod phase=%s", podName, pod.Status.Phase)
		}
		o.Expect(podLogs).To(o.ContainSubstring("already configured and does not need an update"),
			"fencing job must have refused to update stonith (secret never rotated in Pacemaker). Fencing job pod: %s. Full pod logs:\n%s", podName, podLogs)
		o.Expect(podLogs).To(o.ContainSubstring(targetNode.Name),
			"fencing job logs should mention the node %s. Fencing job pod: %s. Full pod logs:\n%s", targetNode.Name, podName, podLogs)

		g.By("Ensuring PacemakerCluster remained healthy: PacemakerHealthCheckDegraded is not set")
		o.Expect(services.WaitForPacemakerHealthCheckHealthy(oc, healthCheckHealthyTimeout, utils.FiveSecondPollInterval)).To(o.Succeed())
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
