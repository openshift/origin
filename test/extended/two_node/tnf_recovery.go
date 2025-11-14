package two_node

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"slices"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	"github.com/openshift/origin/test/extended/two_node/utils/services"
	"github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	memberHasLeftTimeout            = 5 * time.Minute
	memberIsLeaderTimeout           = 20 * time.Minute
	memberRejoinedLearnerTimeout    = 10 * time.Minute
	memberPromotedVotingTimeout     = 15 * time.Minute
	networkDisruptionDuration       = 15 * time.Second
	vmRestartTimeout                = 5 * time.Minute
	vmUngracefulShutdownTimeout     = 30 * time.Second // Ungraceful shutdown is typically fast
	vmGracefulShutdownTimeout       = 10 * time.Minute // Graceful shutdown is typically slow
	membersHealthyAfterDoubleReboot = 30 * time.Minute // It takes into account full VM reboot and Etcd member healthy
	pollInterval                    = 5 * time.Second
)

type hypervisorExtendedConfig struct {
	HypervisorConfig         core.SSHConfig
	HypervisorKnownHostsPath string
}

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Disruptive] Two Node with Fencing etcd recovery", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = util.NewCLIWithoutNamespace("").AsAdmin()
		etcdClientFactory    *helpers.EtcdClientFactoryImpl
		peerNode, targetNode corev1.Node
	)

	g.BeforeEach(func() {
		skipIfNotTopology(oc, v1.DualReplicaTopologyMode)

		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodes.Items)).To(o.BeNumerically("==", 2), "Expected to find 2 Nodes only")

		etcdClientFactory = helpers.NewEtcdClientFactory(oc.KubeClient())

		skipIfClusterIsNotHealthy(oc, etcdClientFactory, nodes)

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
		err := util.TriggerNodeRebootGraceful(oc.KubeClient(), targetNode.Name)
		o.Expect(err).To(o.BeNil(), "Expected to gracefully shutdown the node without errors")
		time.Sleep(time.Minute)

		g.By(fmt.Sprintf("Ensuring %s leaves the member list (timeout: %v)", targetNode.Name, memberHasLeftTimeout))
		o.Eventually(func() error {
			return helpers.EnsureMemberRemoved(g.GinkgoT(), etcdClientFactory, targetNode.Name)
		}, memberHasLeftTimeout, pollInterval).ShouldNot(o.HaveOccurred())

		g.By(fmt.Sprintf("Ensuring that %s is a healthy voting member and adds %s back as learner (timeout: %v)", peerNode.Name, targetNode.Name, memberIsLeaderTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, false, true, // targetNode expected started == false, learner == true
			memberIsLeaderTimeout, pollInterval)

		g.By(fmt.Sprintf("Ensuring %s rejoins as learner (timeout: %v)", targetNode.Name, memberRejoinedLearnerTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, true, true, // targetNode expected started == true, learner == true
			memberRejoinedLearnerTimeout, pollInterval)

		g.By(fmt.Sprintf("Ensuring %s node is promoted back as voting member (timeout: %v)", targetNode.Name, memberPromotedVotingTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, true, false, // targetNode expected started == true, learner == false
			memberPromotedVotingTimeout, pollInterval)
	})

	g.It("should recover from ungraceful node shutdown with etcd member re-addition", func() {
		// Note: In ungraceful shutdown, the targetNode is forcibly shut down while
		// the peerNode remains running and becomes the etcd leader.
		survivedNode := peerNode
		g.GinkgoT().Printf("Randomly selected %s (%s) to be shut down and %s (%s) to take the lead\n",
			targetNode.Name, targetNode.Status.Addresses[0].Address, peerNode.Name, peerNode.Status.Addresses[0].Address)
		g.By(fmt.Sprintf("Shutting down %s ungracefully in 1 minute", targetNode.Name))
		err := util.TriggerNodeRebootUngraceful(oc.KubeClient(), targetNode.Name)
		o.Expect(err).To(o.BeNil(), "Expected to ungracefully shutdown the node without errors", targetNode.Name, err)
		time.Sleep(1 * time.Minute)

		g.By(fmt.Sprintf("Ensuring that %s added %s back as learner (timeout: %v)", peerNode.Name, targetNode.Name, memberIsLeaderTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, false, true, // targetNode expected started == false, learner == true
			memberIsLeaderTimeout, pollInterval)

		g.By(fmt.Sprintf("Ensuring %s rejoins as learner (timeout: %v)", targetNode.Name, memberRejoinedLearnerTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, true, true, // targetNode expected started == true, learner == true
			memberRejoinedLearnerTimeout, pollInterval)

		g.By(fmt.Sprintf("Ensuring %s node is promoted back as voting member (timeout: %v)", targetNode.Name, memberPromotedVotingTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&survivedNode,
			&targetNode, true, false, // targetNode expected started == true, learner == false
			memberPromotedVotingTimeout, pollInterval)
	})

	g.It("should recover from network disruption with etcd member re-addition", func() {
		// Note: In network disruption, the targetNode runs the disruption command that
		// isolates the nodes from each other, creating a split-brain where pacemaker
		// determines which node gets fenced and which becomes the etcd leader.
		g.GinkgoT().Printf("Randomly selected %s (%s) to run the network disruption command\n", targetNode.Name, targetNode.Status.Addresses[0].Address)
		g.By(fmt.Sprintf("Blocking network communication between %s and %s for %v ", targetNode.Name, peerNode.Name, networkDisruptionDuration))
		command, err := util.TriggerNetworkDisruption(oc.KubeClient(), &targetNode, &peerNode, networkDisruptionDuration)
		o.Expect(err).To(o.BeNil(), "Expected to disrupt network without errors")
		g.GinkgoT().Printf("command: '%s'\n", command)

		g.By(fmt.Sprintf("Ensuring cluster recovery with proper leader/learner roles after network disruption (timeout: %v)", memberIsLeaderTimeout))
		// Note: The fenced node may recover quickly and already be started when we get
		// the first etcd membership. This is valid behavior, so we capture the learner's
		// state and adapt the test accordingly.
		leaderNode, learnerNode, learnerStarted := validateEtcdRecoveryStateWithoutAssumingLeader(oc, etcdClientFactory,
			&peerNode, &targetNode, memberIsLeaderTimeout, pollInterval)

		if learnerStarted {
			g.GinkgoT().Printf("Learner node '%s' already started as learner\n", learnerNode.Name)
		} else {
			g.By(fmt.Sprintf("Ensuring '%s' rejoins as learner (timeout: %v)", learnerNode.Name, memberRejoinedLearnerTimeout))
			validateEtcdRecoveryState(oc, etcdClientFactory,
				leaderNode,
				learnerNode, true, true, // targetNode expected started == true, learner == true
				memberRejoinedLearnerTimeout, pollInterval)
		}

		g.By(fmt.Sprintf("Ensuring learner node '%s' is promoted back as voting member (timeout: %v)", learnerNode.Name, memberPromotedVotingTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			leaderNode,
			learnerNode, true, false, // targetNode expected started == true, learner == false
			memberPromotedVotingTimeout, pollInterval)
	})

	g.It("should recover from a double node failure (cold-boot) [Skipped:KnownIssue]", func() {
		// Note: In a double node failure both nodes have the same role, hence we
		// will call them just NodeA and NodeB
		// Currently skipped due to OCPBUGS-59238: rapid podman-etcd restart fails on unpatched clusters
		nodeA := peerNode
		nodeB := targetNode
		c, vmA, vmB, err := setupMinimalTestEnvironment(oc, &nodeA, &nodeB)
		o.Expect(err).To(o.BeNil(), "Expected to setup test environment without error")

		dataPair := []vmNodePair{
			{vmA, nodeA.Name},
			{vmB, nodeB.Name},
		}

		defer restartVms(dataPair, c)

		g.By("Simulating double node failure: stopping both nodes' VMs")
		// First, stop all VMs
		for _, d := range dataPair {
			err := services.VirshDestroyVM(d.vm, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
			o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to stop VM %s (node: %s)", d.vm, d.node))
		}
		// Then, wait for all to reach shut off state
		for _, d := range dataPair {
			err := services.WaitForVMState(d.vm, services.VMStateShutOff, vmUngracefulShutdownTimeout, pollInterval, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
			o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected VM %s (node: %s) to reach shut off state in %s timeout", d.vm, d.node, vmUngracefulShutdownTimeout))
		}

		g.By("Restarting both nodes")
		restartVms(dataPair, c)

		g.By(fmt.Sprintf("Waiting both etcd members to become healthy (timeout: %v)", membersHealthyAfterDoubleReboot))
		// Both nodes are expected to be healthy voting members. The order of nodes passed to the validation function does not matter.
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&nodeA,
			&nodeB, true, false,
			membersHealthyAfterDoubleReboot, pollInterval)
	})

	g.It("should recover from double graceful node shutdown (cold-boot) [Skipped:KnownIssue]", func() {
		// Note: Both nodes are gracefully shut down, then both restart
		// Currently skipped due to OCPBUGS-59238: rapid podman-etcd restart fails on unpatched clusters
		nodeA := peerNode
		nodeB := targetNode
		g.GinkgoT().Printf("Testing double node graceful shutdown for %s and %s\n", nodeA.Name, nodeB.Name)

		c, vmA, vmB, err := setupMinimalTestEnvironment(oc, &nodeA, &nodeB)
		o.Expect(err).To(o.BeNil(), "Expected to setup test environment without error")

		dataPair := []vmNodePair{
			{vmA, nodeA.Name},
			{vmB, nodeB.Name},
		}

		defer restartVms(dataPair, c)

		g.By(fmt.Sprintf("Gracefully shutting down both nodes at the same time (timeout: %v)", vmGracefulShutdownTimeout))
		for _, d := range dataPair {
			innerErr := services.VirshShutdownVM(d.vm, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
			o.Expect(innerErr).To(o.BeNil(), fmt.Sprintf("Expected to gracefully shutdown VM %s (node: %s)", d.vm, d.node))
		}

		for _, d := range dataPair {
			innerErr := services.WaitForVMState(d.vm, services.VMStateShutOff, vmGracefulShutdownTimeout, pollInterval, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
			o.Expect(innerErr).To(o.BeNil(), fmt.Sprintf("Expected VM %s (node: %s) to reach shut off state", d.vm, d.node))
		}

		g.By("Restarting both nodes")
		restartVms(dataPair, c)

		g.By(fmt.Sprintf("Waiting both etcd members to become healthy (timeout: %v)", membersHealthyAfterDoubleReboot))
		// Both nodes are expected to be healthy voting members. The order of nodes passed to the validation function does not matter.
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&nodeA,
			&nodeB, true, false,
			membersHealthyAfterDoubleReboot, pollInterval)
	})

	g.It("should recover from sequential graceful node shutdowns (cold-boot) [Skipped:KnownIssue]", func() {
		// Note: First node is gracefully shut down, then the second, then both restart
		// Currently skipped due to OCPBUGS-59238: rapid podman-etcd restart fails on unpatched clusters
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
			membersHealthyAfterDoubleReboot, pollInterval)
	})

	g.It("should recover from graceful shutdown followed by ungraceful node failure (cold-boot) [Skipped:KnownIssue]", func() {
		// Note: First node is gracefully shut down, then the survived node fails ungracefully
		// Currently skipped due to OCPBUGS-59238: rapid podman-etcd restart fails on unpatched clusters
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

		defer restartVms(dataPair, c)

		g.By(fmt.Sprintf("Gracefully shutting down VM %s (node: %s)", vmFirstToShutdown, firstToShutdown.Name))
		err = vmShutdownAndWait(VMShutdownModeGraceful, vmFirstToShutdown, c)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected VM %s to reach shut off state", vmFirstToShutdown))

		g.By(fmt.Sprintf("Waiting for %s to recover the etcd cluster standalone (timeout: %v)", secondToShutdown.Name, memberIsLeaderTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&secondToShutdown,
			&firstToShutdown, false, true, // expected started == false, learner == true
			memberIsLeaderTimeout, pollInterval)

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
			membersHealthyAfterDoubleReboot, pollInterval)
	})
})

func getMembers(etcdClientFactory helpers.EtcdClientCreator) ([]*etcdserverpb.Member, error) {
	etcdClient, closeFn, err := etcdClientFactory.NewEtcdClient()
	if err != nil {
		return []*etcdserverpb.Member{}, errors.Wrap(err, "could not get a etcd client")
	}
	defer closeFn()

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()
	m, err := etcdClient.MemberList(ctx)
	if err != nil {
		return []*etcdserverpb.Member{}, errors.Wrap(err, "could not get the member list")
	}
	return m.Members, nil
}

func getMemberState(node *corev1.Node, members []*etcdserverpb.Member) (started, learner bool, err error) {
	// Etcd members that have been added to the member list but haven't
	// joined yet will have an empty Name field. We can match them via Peer URL.
	hostPort := net.JoinHostPort(node.Status.Addresses[0].Address, "2380")
	peerURL := fmt.Sprintf("https://%s", hostPort)
	var found bool
	for _, m := range members {
		if m.Name == node.Name {
			found = true
			started = true
			learner = m.IsLearner
			break
		}
		if slices.Contains(m.PeerURLs, peerURL) {
			found = true
			learner = m.IsLearner
			break
		}
	}
	if !found {
		return false, false, fmt.Errorf("could not find node %v via peer URL %s", node.Name, peerURL)
	}
	return started, learner, nil
}

// validateEtcdRecoveryState polls the etcd cluster until the members match the expected state or a timeout is reached.
//
// This function assumes that the first node argument is always expected to be a healthy, voting member (isStarted=true, isLearner=false).
// It validates the state of the second node argument against the provided `isTargetNodeStartedExpected` and `isTargetNodeLearnerExpected` booleans.
//
// When both nodes are expected to be healthy voting members, the order of the node arguments is interchangeable.
func validateEtcdRecoveryState(
	oc *util.CLI, e *helpers.EtcdClientFactoryImpl,
	survivedNode, targetNode *corev1.Node,
	isTargetNodeStartedExpected, isTargetNodeLearnerExpected bool,
	timeout, pollInterval time.Duration) {
	o.EventuallyWithOffset(1, func() error {
		members, err := getMembers(e)
		if err != nil {
			return err
		}
		if len(members) != 2 {
			return fmt.Errorf("Not enough members")
		}

		if isStarted, isLearner, err := getMemberState(survivedNode, members); err != nil {
			return err
		} else if !isStarted || isLearner {
			return fmt.Errorf("Expected survived node %s to be started and voting member, got this membership instead: %+v",
				survivedNode.Name, members)
		}

		isStarted, isLearner, err := getMemberState(targetNode, members)
		if err != nil {
			return err
		}

		// lazy check node reboot: make API calls only if and when needed
		var hasTargetNodeRebooted bool
		lazyCheckReboot := func() bool {
			// return cached value only if the node has already rebooted during this test
			if !hasTargetNodeRebooted {
				var checkErr error
				hasTargetNodeRebooted, checkErr = hasNodeRebooted(oc, targetNode)
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
				g.GinkgoT().Logf("Target node %s has re-started already", targetNode.Name)
			} else {
				return fmt.Errorf("Expected target node %s to have status started==%v (got %v). Full membership: %+v",
					targetNode.Name, isTargetNodeStartedExpected, isStarted, members)
			}
		}
		if isTargetNodeLearnerExpected != isLearner {
			if isTargetNodeLearnerExpected && lazyCheckReboot() { // expected "learner", but "voter" already after a reboot
				g.GinkgoT().Logf("Target node %s was promoted to voter already", targetNode.Name)
			} else {
				return fmt.Errorf("Expected target node %s to have status started==%v (got %v) and voting member==%v (got %v). Full membership: %+v",
					targetNode.Name, isTargetNodeStartedExpected, isStarted, isTargetNodeLearnerExpected, isLearner, members)
			}
		}

		g.GinkgoT().Logf("SUCCESS: got membership: %+v", members)
		return nil
	}, timeout, pollInterval).ShouldNot(o.HaveOccurred())
}

func validateEtcdRecoveryStateWithoutAssumingLeader(
	oc *util.CLI, e *helpers.EtcdClientFactoryImpl,
	nodeA, nodeB *corev1.Node,
	timeout, pollInterval time.Duration) (leaderNode, learnerNode *corev1.Node, learnerStarted bool) {

	o.EventuallyWithOffset(1, func() error {
		members, err := getMembers(e)
		if err != nil {
			return err
		}
		if len(members) != 2 {
			return fmt.Errorf("expected 2 members, got %d", len(members))
		}

		// Get state for both nodes first
		startedA, learnerA, err := getMemberState(nodeA, members)
		if err != nil {
			return fmt.Errorf("failed to get state for node %s: %v", nodeA.Name, err)
		}

		startedB, learnerB, err := getMemberState(nodeB, members)
		if err != nil {
			return fmt.Errorf("failed to get state for node %s: %v", nodeB.Name, err)
		}

		// Then, evaluate the possible combinations
		if !startedA && !startedB {
			return fmt.Errorf("etcd members have not started yet")
		}

		// This should not happen
		if learnerA && learnerB {
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
			hasNodeARebooted, err := hasNodeRebooted(oc, nodeA)
			if err != nil {
				return err
			}
			hasNodeBRebooted, err := hasNodeRebooted(oc, nodeB)
			if err != nil {
				return err
			}

			if hasNodeARebooted != hasNodeBRebooted {
				framework.Logf("both nodes are non-learners, but only one has rebooted, hence the cluster has indeed recovered from a disruption")
				// the rebooted node is the learner
				learnerA = hasNodeARebooted
				learnerB = hasNodeBRebooted
			} else if hasNodeARebooted && hasNodeBRebooted {
				return fmt.Errorf("both nodes rebooted. This indicates a cluster disruption beyond the expected single-node failure")
			} else {
				return fmt.Errorf("both nodes are non-learners (should have exactly one learner): %s(started=%v, learner=%v), %s(started=%v, learner=%v)", nodeA.Name, startedA, learnerA, nodeB.Name, startedB, learnerB)
			}
		}

		// Once we get one leader and one learner, we don't care if the latter has started already, but the first must
		// already been started
		leaderStarted := (startedA && !learnerA) || (startedB && !learnerB)
		if !leaderStarted {
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

		g.GinkgoT().Logf("SUCCESS: Leader is %s, learner is %s (started=%v)",
			leaderNode.Name, learnerNode.Name, learnerStarted)

		return nil
	}, timeout, pollInterval).ShouldNot(o.HaveOccurred())

	return leaderNode, learnerNode, learnerStarted
}

// setupMinimalTestEnvironment validates prerequisites and gathers required information for double node failure test
func setupMinimalTestEnvironment(oc *util.CLI, nodeA, nodeB *corev1.Node) (c hypervisorExtendedConfig, vmNameNodeA, vmNameNodeB string, err error) {
	if !util.HasHypervisorConfig() {
		services.PrintHypervisorConfigUsage()
		err = fmt.Errorf("no hypervisor configuration available")
		return
	}

	sshConfig := util.GetHypervisorConfig()
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

	return services.WaitForVMState(vm, services.VMStateShutOff, timeout, pollInterval, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
}

func restartVms(dataPair []vmNodePair, c hypervisorExtendedConfig) {
	// Start all VMs asynchronously
	for _, d := range dataPair {
		if err := services.VirshStartVM(d.vm, &c.HypervisorConfig, c.HypervisorKnownHostsPath); err != nil {
			fmt.Fprintf(g.GinkgoWriter, "Warning: failed to restart VM %s during cleanup: %v\n", d.vm, err)
		}
	}

	// Wait for all VMs to be running
	for _, d := range dataPair {
		err := services.WaitForVMState(d.vm, services.VMStateRunning, vmRestartTimeout, pollInterval, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected VM %s (node: %s) to start in %s timeout", d.vm, d.node, vmRestartTimeout))
	}
}
