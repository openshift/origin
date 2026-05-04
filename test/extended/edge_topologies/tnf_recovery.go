package edge_topologies

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/edge_topologies/utils"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/apis"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/core"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/services"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	nodeIsHealthyTimeout            = time.Minute
	etcdOperatorIsHealthyTimeout    = time.Minute
	memberIsLeaderTimeout           = 20 * time.Minute
	memberRejoinedLearnerTimeout    = 20 * time.Minute
	memberPromotedVotingTimeout     = 15 * time.Minute
	networkDisruptionDuration       = 15 * time.Second
	vmRestartTimeout                = 5 * time.Minute
	vmUngracefulShutdownTimeout     = 30 * time.Second // Ungraceful VM shutdown is typically fast
	vmGracefulShutdownTimeout       = 10 * time.Minute // Graceful VM shutdown is typically slow
	membersHealthyAfterDoubleReboot = 30 * time.Minute // Includes full VM reboot and etcd member healthy
	progressLogInterval             = time.Minute      // Target interval for progress logging
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

// logRecoveryPath reads the surviving node's journal to detect whether the
// graceful shutdown used the clean-leave or force-new-cluster recovery path.
// Emits a structured [RecoveryPath] log line for CI tracking.
func logRecoveryPath(oc *exutil.CLI, survivedNode, targetNode *corev1.Node) {
	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, survivedNode.Name, "openshift-etcd",
		"bash", "-c", "journalctl --since '60 min ago' --no-pager | grep -m1 'force.new.cluster' || true")
	if err != nil {
		framework.Logf("[sig-etcd][RecoveryPath] node=%s path=unknown: journal check failed: %v", targetNode.Name, err)
		return
	}
	if strings.TrimSpace(output) != "" {
		framework.Logf("[sig-etcd][RecoveryPath] node=%s path=force-new-cluster journal=%q",
			targetNode.Name, strings.TrimSpace(output))
	} else {
		framework.Logf("[sig-etcd][RecoveryPath] node=%s path=clean-leave", targetNode.Name)
	}
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

		// Log concise pcs and etcd status after every test (pass or fail) via SSH.
		// Complements deferDiagnosticsOnFailure which gathers verbose diagnostics only on failure.
		g.DeferCleanup(func() {
			logFinalClusterStatus([]corev1.Node{peerNode, targetNode})
		})
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

		g.By(fmt.Sprintf("Checking recovery path for %s from %s journal", targetNode.Name, survivedNode.Name))
		logRecoveryPath(oc, &survivedNode, &targetNode)
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

		g.By("Restarting both nodes")
		restartVms(dataPair, c)

		g.By(fmt.Sprintf("Waiting both etcd members to become healthy (timeout: %v)", membersHealthyAfterDoubleReboot))
		// Both nodes are expected to be healthy voting members. The order of nodes passed to the validation function does not matter.
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&nodeA,
			&nodeB, true, false,
			membersHealthyAfterDoubleReboot, utils.FiveSecondPollInterval)
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

		g.By("Restarting both nodes")
		restartVms(dataPair, c)

		g.By(fmt.Sprintf("Waiting both etcd members to become healthy (timeout: %v)", membersHealthyAfterDoubleReboot))
		// Both nodes are expected to be healthy voting members. The order of nodes passed to the validation function does not matter.
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&firstToShutdown,
			&secondToShutdown, true, false,
			membersHealthyAfterDoubleReboot, utils.FiveSecondPollInterval)
	})

	g.It("should recover from BMC credential rotation with fencing", func() {
		bmcNode := targetNode
		survivedNode := peerNode

		ns, secretName, originalPassword, err := apis.RotateNodeBMCPassword(oc, &bmcNode)
		o.Expect(err).ToNot(o.HaveOccurred(), "expected to rotate BMC credentials without error")

		defer func() {
			if err := apis.RestoreBMCPassword(oc, ns, secretName, originalPassword); err != nil {
				fmt.Fprintf(g.GinkgoWriter,
					"Warning: failed to restore original BMC password in %s/%s: %v\n",
					ns, secretName, err)
			}
		}()
		g.By("Ensuring etcd members remain healthy after BMC credential rotation")
		o.Eventually(func() error {
			if err := helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, survivedNode.Name); err != nil {
				return err
			}
			if err := helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, bmcNode.Name); err != nil {
				return err
			}
			return nil
		}, nodeIsHealthyTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred(), "etcd members should be healthy after BMC credential rotation")

		g.By(fmt.Sprintf("Triggering a fencing-style network disruption between %s and %s", bmcNode.Name, survivedNode.Name))
		command, err := exutil.TriggerNetworkDisruption(oc.KubeClient(), &bmcNode, &survivedNode, networkDisruptionDuration)
		o.Expect(err).To(o.BeNil(), "Expected to disrupt network without errors")
		framework.Logf("network disruption command: %q", command)

		g.By(fmt.Sprintf("Ensuring cluster recovery with proper leader/learner roles after BMC credential rotation + network disruption (timeout: %v)", memberIsLeaderTimeout))
		leaderNode, learnerNode, learnerStarted := validateEtcdRecoveryStateWithoutAssumingLeader(oc, etcdClientFactory,
			&survivedNode, &bmcNode, memberIsLeaderTimeout, utils.FiveSecondPollInterval)

		if learnerStarted {
			framework.Logf("Learner node %q already started as learner after disruption", learnerNode.Name)
		} else {
			g.By(fmt.Sprintf("Ensuring '%s' rejoins as learner (timeout: %v)", learnerNode.Name, memberRejoinedLearnerTimeout))
			validateEtcdRecoveryState(oc, etcdClientFactory,
				leaderNode,
				learnerNode, true, true,
				memberRejoinedLearnerTimeout, utils.FiveSecondPollInterval)
		}

		g.By(fmt.Sprintf("Ensuring learner node '%s' is promoted back as voting member (timeout: %v)", learnerNode.Name, memberPromotedVotingTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			leaderNode,
			learnerNode, true, false,
			memberPromotedVotingTimeout, utils.FiveSecondPollInterval)
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

	g.It("should leave a backup container behind for debugging when etcd container crashes", func() {
		survivedNode := peerNode

		g.By("Recording epoch timestamp before reboot")
		epochStr, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, targetNode.Name, "openshift-etcd",
			"bash", "-c", "date +%s")
		o.Expect(err).To(o.BeNil())
		rebootEpoch := strings.TrimSpace(epochStr)

		g.By(fmt.Sprintf("Cleaning up any stale etcd-previous container on %s", targetNode.Name))
		_, _ = exutil.DebugNodeRetryWithOptionsAndChroot(oc, targetNode.Name, "openshift-etcd",
			"bash", "-c", "podman rm -f etcd-previous 2>/dev/null || true")

		g.By(fmt.Sprintf("Removing /var/lib/etcd/pod.yaml on %s", targetNode.Name))
		_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, targetNode.Name, "openshift-etcd",
			"bash", "-c", "rm -f /var/lib/etcd/pod.yaml")
		o.Expect(err).To(o.BeNil(), "Expected to remove pod.yaml without error")

		g.By(fmt.Sprintf("Rebooting %s ungracefully", targetNode.Name))
		err = exutil.TriggerNodeRebootUngraceful(oc.KubeClient(), targetNode.Name)
		o.Expect(err).To(o.BeNil(), "Expected to trigger ungraceful reboot without error")
		time.Sleep(time.Minute)

		g.By(fmt.Sprintf("Ensuring that %s added %s back as learner (timeout: %v)", survivedNode.Name, targetNode.Name, memberIsLeaderTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, false, true,
			memberIsLeaderTimeout, utils.FiveSecondPollInterval)

		g.By(fmt.Sprintf("Ensuring %s rejoins as learner (timeout: %v)", targetNode.Name, memberRejoinedLearnerTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, true, true,
			memberRejoinedLearnerTimeout, utils.FiveSecondPollInterval)

		g.By(fmt.Sprintf("Ensuring %s node is promoted back as voting member (timeout: %v)", targetNode.Name, memberPromotedVotingTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, true, false,
			memberPromotedVotingTimeout, utils.FiveSecondPollInterval)

		g.By(fmt.Sprintf("Verifying etcd container is running on %s", targetNode.Name))
		got, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, targetNode.Name, "openshift-etcd",
			strings.Split(ensurePodmanEtcdContainerIsRunning, " ")...)
		o.Expect(err).To(o.BeNil())
		o.Expect(got).To(o.Equal("'true'"), fmt.Sprintf("expected etcd container running on %s", targetNode.Name))

		g.By(fmt.Sprintf("Verifying etcd-previous container exists on %s", targetNode.Name))
		prevOutput, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, targetNode.Name, "openshift-etcd",
			"bash", "-c", "podman ps -a --format '{{.Names}}' | grep -m1 etcd-previous")
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("expected etcd-previous container to exist on %s", targetNode.Name))
		o.Expect(strings.TrimSpace(prevOutput)).To(o.Equal("etcd-previous"),
			fmt.Sprintf("expected etcd-previous container on %s", targetNode.Name))

		g.By(fmt.Sprintf("Verifying pod.yaml was recreated on %s via pacemaker log", targetNode.Name))
		_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, targetNode.Name, "openshift-etcd",
			"bash", "-c", fmt.Sprintf("journalctl -u pacemaker --since=@%s --no-pager | grep -m1 -i 'a new working copy of /etc/kubernetes/static-pod-resources/etcd-certs/configmaps/external-etcd-pod/pod.yaml was created'", rebootEpoch))
		o.Expect(err).To(o.BeNil(), "Expected pacemaker log to contain pod.yaml recreation entry after reboot")

	})

	g.It("should recover after simultaneous graceful shutdown of both nodes", func() {
		g.GinkgoT().Printf("Gracefully rebooting both nodes: %s and %s\n",
			targetNode.Name, peerNode.Name)

		g.By(fmt.Sprintf("Triggering graceful reboot on %s", targetNode.Name))
		err := exutil.TriggerNodeRebootGraceful(oc.KubeClient(), targetNode.Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to trigger graceful reboot on %s without error", targetNode.Name))

		g.By(fmt.Sprintf("Triggering graceful reboot on %s", peerNode.Name))
		err = exutil.TriggerNodeRebootGraceful(oc.KubeClient(), peerNode.Name)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to trigger graceful reboot on %s without error", peerNode.Name))

		g.By("Waiting for graceful shutdown to take effect (shutdown -r 1 schedules reboot in 1 minute)")
		time.Sleep(90 * time.Second)

		g.By(fmt.Sprintf("Waiting for both etcd members to become healthy (timeout: %v)", membersHealthyAfterDoubleReboot))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&targetNode,
			&peerNode, true, false,
			membersHealthyAfterDoubleReboot, utils.FiveSecondPollInterval)

		g.By("Verifying etcd containers are running on both nodes")
		for _, node := range []corev1.Node{targetNode, peerNode} {
			got, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, node.Name, "openshift-etcd",
				strings.Split(ensurePodmanEtcdContainerIsRunning, " ")...)
			o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected no error checking etcd on %s", node.Name))
			o.Expect(got).To(o.Equal("'true'"), fmt.Sprintf("Expected etcd container running on %s", node.Name))
		}
	})

	g.It("should compute etcd revision bump after kernel panic recovery", func() {
		// Note: This test triggers a kernel panic on one node via sysrq trigger, then verifies
		// the surviving node computes the etcd revision bump as floor(maxRaftIndex * 0.2) per
		// compute_bump_revision in podman-etcd (https://github.com/ClusterLabs/resource-agents/pull/2087).
		// Requires resource-agents >= 4.10.0-71.el9_6.13 (RHEL 9) or >= 4.16.0-33.el10 (RHEL 10).
		survivedNode := peerNode

		g.By("Logging resource-agents RPM version")
		raVersion, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, survivedNode.Name, "openshift-etcd",
			"bash", "-c", "rpm -q resource-agents")
		o.Expect(err).To(o.BeNil())
		framework.Logf("Installed resource-agents: %s (requires >= 4.10.0-71.el9_6.13 for RHEL 9 or >= 4.16.0-33.el10 for RHEL 10)",
			strings.TrimSpace(raVersion))

		g.By("Recording timestamp before crash")
		timestampStr, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, survivedNode.Name, "openshift-etcd",
			"bash", "-c", "date -u '+%Y-%m-%d %H:%M:%S'")
		o.Expect(err).To(o.BeNil())
		crashTimestamp := strings.TrimSpace(timestampStr)

		g.By(fmt.Sprintf("Triggering kernel panic on %s via sysrq trigger", targetNode.Name))
		disruptPodName := fmt.Sprintf("disrupt-%s-0", targetNode.Name)
		err = exutil.TriggerKernelPanic(oc.KubeClient(), targetNode.Name)
		o.Expect(err).To(o.BeNil(), "Expected to trigger kernel panic without error")

		g.By(fmt.Sprintf("Ensuring that %s added %s back as learner (timeout: %v)", survivedNode.Name, targetNode.Name, memberIsLeaderTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, false, true,
			memberIsLeaderTimeout, utils.FiveSecondPollInterval)

		g.By("Cleaning up disruption pod")
		gracePeriod := int64(0)
		_ = oc.KubeClient().CoreV1().Pods("kube-system").Delete(context.Background(),
			disruptPodName, metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod})

		g.By("Reading bump-amount from journal log on survived node")
		var journalBump int
		o.Eventually(func() error {
			journalOutput, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, survivedNode.Name, "openshift-etcd",
				"bash", "-c", fmt.Sprintf("journalctl -u pacemaker --since '%s' | grep 'bump-amount' | tail -1", crashTimestamp))
			if err != nil {
				return fmt.Errorf("failed to read journal: %v", err)
			}
			// Parse bump-amount from JSON log: "bump-amount":21391
			matches := regexp.MustCompile(`"bump-amount":(\d+)`).FindStringSubmatch(journalOutput)
			if len(matches) < 2 {
				return fmt.Errorf("bump-amount not found in journal output: %s", journalOutput)
			}
			journalBump, err = strconv.Atoi(matches[1])
			if err != nil {
				return fmt.Errorf("failed to parse bump-amount %q: %v", matches[1], err)
			}
			return nil
		}, memberIsLeaderTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred())

		g.By("Verifying force-new-cluster-bump-amount in config.yaml matches journal bump-amount")
		var configBump int
		o.Eventually(func() error {
			bumpAmountStr, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, survivedNode.Name, "openshift-etcd",
				"bash", "-c", "grep 'force-new-cluster-bump-amount:' /var/lib/etcd/config.yaml | awk '{print $2}'")
			if err != nil {
				return fmt.Errorf("failed to read bump amount: %v", err)
			}
			configBump, err = strconv.Atoi(strings.TrimSpace(bumpAmountStr))
			if err != nil {
				return fmt.Errorf("failed to parse bump amount %q: %v", bumpAmountStr, err)
			}
			return nil
		}, memberIsLeaderTimeout, utils.FiveSecondPollInterval).ShouldNot(o.HaveOccurred())
		o.Expect(configBump).To(o.Equal(journalBump),
			fmt.Sprintf("config.yaml bump-amount %d should match journal bump-amount %d", configBump, journalBump))

		g.By("Independently verifying bump amount is approximately floor(maxRaftIndex * 0.2)")
		raftIndexStr, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, survivedNode.Name, "openshift-etcd",
			"bash", "-c", "jq -r '.maxRaftIndex' /var/lib/etcd/revision.json")
		o.Expect(err).To(o.BeNil())
		maxRaftIndex, err := strconv.Atoi(strings.TrimSpace(raftIndexStr))
		o.Expect(err).To(o.BeNil())
		// maxRaftIndex may have grown since compute_bump_revision ran, so configBump
		// should be <= floor(currentMaxRaftIndex * 0.2) and > 0
		o.Expect(configBump).To(o.BeNumerically(">", 0))
		o.Expect(configBump).To(o.BeNumerically("<=", int(float64(maxRaftIndex)*0.2)),
			fmt.Sprintf("bump %d should be <= floor(%d * 0.2) = %d", configBump, maxRaftIndex, int(float64(maxRaftIndex)*0.2)))

		g.By(fmt.Sprintf("Ensuring %s rejoins as learner (timeout: %v)", targetNode.Name, memberRejoinedLearnerTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, true, true,
			memberRejoinedLearnerTimeout, utils.FiveSecondPollInterval)

		g.By(fmt.Sprintf("Ensuring %s node is promoted back as voting member (timeout: %v)", targetNode.Name, memberPromotedVotingTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, true, false,
			memberPromotedVotingTimeout, utils.FiveSecondPollInterval)
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
	if sshConfig == nil {
		err = fmt.Errorf("failed to parse hypervisor config")
		return
	}
	c.HypervisorConfig.IP = sshConfig.HypervisorIP
	c.HypervisorConfig.User = sshConfig.SSHUser
	c.HypervisorConfig.PrivateKeyPath = sshConfig.PrivateKeyPath

	// Validate that the private key file exists
	if _, err = os.Stat(c.HypervisorConfig.PrivateKeyPath); err != nil {
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

// logFinalClusterStatus logs pcs status and etcd member list via SSH after every test
// (pass or fail). Uses the hypervisor SSH path because the Kubernetes API may not be
// available after a recovery test. Errors are logged but never fail the test.
func logFinalClusterStatus(nodes []corev1.Node) {
	if !exutil.HasHypervisorConfig() {
		return
	}

	sshConfig := exutil.GetHypervisorConfig()
	if sshConfig == nil {
		framework.Logf("Skipping final cluster status: failed to parse hypervisor config")
		return
	}
	hypervisorConfig := core.SSHConfig{
		IP:             sshConfig.HypervisorIP,
		User:           sshConfig.SSHUser,
		PrivateKeyPath: sshConfig.PrivateKeyPath,
	}

	if _, err := os.Stat(hypervisorConfig.PrivateKeyPath); err != nil {
		framework.Logf("Skipping final cluster status: cannot access private key at %s: %v", hypervisorConfig.PrivateKeyPath, err)
		return
	}

	knownHostsPath, err := core.PrepareLocalKnownHostsFile(&hypervisorConfig)
	if err != nil {
		framework.Logf("Skipping final cluster status: failed to prepare known hosts: %v", err)
		return
	}

	framework.Logf("========== FINAL CLUSTER STATUS ==========")

	for _, node := range nodes {
		nodeIP := utils.GetNodeInternalIP(&node)
		if nodeIP == "" {
			framework.Logf("Skipping node %s: no internal IP", node.Name)
			continue
		}

		remoteKnownHostsPath, err := core.PrepareRemoteKnownHostsFile(nodeIP, &hypervisorConfig, knownHostsPath)
		if err != nil {
			framework.Logf("Failed to prepare remote known hosts for node %s: %v", node.Name, err)
			continue
		}

		// pcs status
		pcsOutput, pcsStderr, pcsErr := services.PcsStatus(nodeIP, &hypervisorConfig, knownHostsPath, remoteKnownHostsPath)
		if pcsErr != nil {
			framework.Logf("Failed to get pcs status from node %s: %v\nstdout: %s\nstderr: %s", node.Name, pcsErr, pcsOutput, pcsStderr)
		} else {
			framework.Logf("pcs status from node %s:\n%s", node.Name, pcsOutput)
		}

		// etcd member list via SSH (-w table is the etcdctl v3 flag for table output)
		etcdOutput, etcdStderr, etcdErr := core.ExecuteRemoteSSHCommand(nodeIP,
			"sudo podman exec etcd etcdctl member list -w table",
			&hypervisorConfig, knownHostsPath, remoteKnownHostsPath)
		if etcdErr != nil {
			framework.Logf("Failed to get etcd member list from node %s: %v\nstdout: %s\nstderr: %s", node.Name, etcdErr, etcdOutput, etcdStderr)
		} else {
			framework.Logf("etcd member list from node %s:\n%s", node.Name, etcdOutput)
		}

		// Only need one successful node for cluster-wide status
		if pcsErr == nil && etcdErr == nil {
			break
		}
	}

	framework.Logf("========== END FINAL CLUSTER STATUS ==========")
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
