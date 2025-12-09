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
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/apis"
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	"github.com/openshift/origin/test/extended/two_node/utils/services"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	nodeIsHealthyTimeout            = time.Minute
	etcdOperatorIsHealthyTimeout    = time.Minute
	memberHasLeftTimeout            = 5 * time.Minute
	memberIsLeaderTimeout           = 10 * time.Minute
	memberRejoinedLearnerTimeout    = 10 * time.Minute
	memberPromotedVotingTimeout     = 15 * time.Minute
	networkDisruptionDuration       = 15 * time.Second
	vmRestartTimeout                = 5 * time.Minute
	vmUngracefulShutdownTimeout     = 30 * time.Second // Ungraceful VM shutdown is typically fast
	membersHealthyAfterDoubleReboot = 15 * time.Minute // It takes into account full VM recovering up to Etcd member healthy
	pollInterval                    = 5 * time.Second
)

type hypervisorExtendedConfig struct {
	HypervisorConfig         core.SSHConfig
	HypervisorKnownHostsPath string
}

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Disruptive][Serial] Two Node with Fencing etcd recovery", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = exutil.NewCLIWithoutNamespace("").AsAdmin()
		etcdClientFactory    *helpers.EtcdClientFactoryImpl
		peerNode, targetNode corev1.Node
	)

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)

		g.By("Verifying etcd cluster operator is healthy before starting test")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy before starting test")

		nodes, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodes.Items)).To(o.BeNumerically("==", 2), "Expected to find 2 Nodes only")

		// Select the first index randomly
		randomIndex := rand.Intn(len(nodes.Items))
		peerNode = nodes.Items[randomIndex]
		// Select the remaining index
		targetNode = nodes.Items[(randomIndex+1)%len(nodes.Items)]
		g.GinkgoT().Printf("Randomly selected %s (%s) to be shut down and %s (%s) to take the lead\n", targetNode.Name, targetNode.Status.Addresses[0].Address, &peerNode.Name, peerNode.Status.Addresses[0].Address)

		kubeClient := oc.KubeClient()
		etcdClientFactory = helpers.NewEtcdClientFactory(kubeClient)

		g.GinkgoT().Printf("Ensure both nodes are healthy before starting the test\n")
		o.Eventually(func() error {
			return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, peerNode.Name)
		}, nodeIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("expect to ensure Node '%s' healthiness without errors", peerNode.Name))

		o.Eventually(func() error {
			return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, targetNode.Name)
		}, nodeIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), fmt.Sprintf("expect to ensure Node '%s' healthiness without errors", targetNode.Name))
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
		err := exutil.TriggerNodeRebootUngraceful(oc.KubeClient(), targetNode.Name)
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
		command, err := exutil.TriggerNetworkDisruption(oc.KubeClient(), &targetNode, &peerNode, networkDisruptionDuration)
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

	g.It("should recover from a double node failure", func() {
		// Note: In a double node failure both nodes have the same role, hence we
		// will call them just NodeA and NodeB
		nodeA := peerNode
		nodeB := targetNode
		c, vmA, vmB, err := setupMinimalTestEnvironment(oc, &nodeA, &nodeB)
		o.Expect(err).To(o.BeNil(), "Expected to setup test environment without error")

		dataPair := []struct {
			vm, node string
		}{
			{vmA, nodeA.Name},
			{vmB, nodeB.Name},
		}

		defer func() {
			for _, d := range dataPair {
				if err := services.VirshStartVM(d.vm, &c.HypervisorConfig, c.HypervisorKnownHostsPath); err != nil {
					fmt.Fprintf(g.GinkgoWriter, "Warning: failed to restart VM %s during cleanup: %v\n", d.vm, err)
				}
			}
		}()

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
		// Start all VMs
		for _, d := range dataPair {
			err := services.VirshStartVM(d.vm, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
			o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected to start VM %s (node: %s)", d.vm, d.node))
		}
		// Wait for all to be running
		for _, d := range dataPair {
			err := services.WaitForVMState(d.vm, services.VMStateRunning, vmUngracefulShutdownTimeout, pollInterval, &c.HypervisorConfig, c.HypervisorKnownHostsPath)
			o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected VM %s (node: %s) to start in %s timeout", d.vm, d.node, vmRestartTimeout))
		}

		g.By(fmt.Sprintf("Waiting both etcd members to become healthy (timeout: %v)", membersHealthyAfterDoubleReboot))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			&nodeA,              // member on node A considered leader, hence started == true, learner == false
			&nodeB, true, false, // member on node B expected started == true, learner == false
			membersHealthyAfterDoubleReboot, pollInterval)
	})

	g.It("should recover from BMC credential rotation with fencing", func() {
		bmcNode := targetNode
		survivedNode := peerNode

		kubeClient := oc.AdminKubeClient()

		ns, secretName, originalPassword, err := apis.RotateNodeBMCPassword(kubeClient, &bmcNode)
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
			if err := helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, survivedNode.Name); err != nil {
				return err
			}
			if err := helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, bmcNode.Name); err != nil {
				return err
			}
			return nil
		}, nodeIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd members should be healthy after BMC credential rotation")

		g.By(fmt.Sprintf("Triggering a fencing-style network disruption between %s and %s", bmcNode.Name, survivedNode.Name))
		command, err := exutil.TriggerNetworkDisruption(oc.KubeClient(), &bmcNode, &survivedNode, networkDisruptionDuration)
		o.Expect(err).To(o.BeNil(), "Expected to disrupt network without errors")
		framework.Logf("network disruption command: %q", command)

		g.By(fmt.Sprintf("Ensuring cluster recovery with proper leader/learner roles after BMC credential rotation + network disruption (timeout: %v)", memberIsLeaderTimeout))
		leaderNode, learnerNode, learnerStarted := validateEtcdRecoveryStateWithoutAssumingLeader(oc, etcdClientFactory,
			&survivedNode, &bmcNode, memberIsLeaderTimeout, pollInterval)

		if learnerStarted {
			framework.Logf("Learner node %q already started as learner after disruption", learnerNode.Name)
		} else {
			g.By(fmt.Sprintf("Ensuring '%s' rejoins as learner (timeout: %v)", learnerNode.Name, memberRejoinedLearnerTimeout))
			validateEtcdRecoveryState(oc, etcdClientFactory,
				leaderNode,
				learnerNode, true, true,
				memberRejoinedLearnerTimeout, pollInterval)
		}

		g.By(fmt.Sprintf("Ensuring learner node '%s' is promoted back as voting member (timeout: %v)", learnerNode.Name, memberPromotedVotingTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			leaderNode,
			learnerNode, true, false,
			memberPromotedVotingTimeout, pollInterval)
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

// ensureEtcdOperatorHealthy checks if the cluster-etcd-operator is healthy before running etcd tests
func ensureEtcdOperatorHealthy(oc *exutil.CLI) error {
	g.By("Checking etcd ClusterOperator status")
	etcdOperator, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(context.Background(), "etcd", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to retrieve etcd ClusterOperator: %v", err)
	}

	// Check if etcd operator is Available
	if !utils.IsClusterOperatorAvailable(etcdOperator) {
		return fmt.Errorf("etcd ClusterOperator is not Available")
	}

	// Check if etcd operator is not Degraded
	if utils.IsClusterOperatorDegraded(etcdOperator) {
		degraded := findClusterOperatorCondition(etcdOperator.Status.Conditions, v1.OperatorDegraded)
		return fmt.Errorf("etcd ClusterOperator is Degraded: %s", degraded.Message)
	}

	// Check if etcd operator is not Progressing (optional - might be ok during normal operations)
	progressing := findClusterOperatorCondition(etcdOperator.Status.Conditions, v1.OperatorProgressing)
	if progressing != nil && progressing.Status == v1.ConditionTrue {
		g.GinkgoT().Logf("Warning: etcd ClusterOperator is Progressing: %s", progressing.Message)
	}

	g.By("Checking etcd pods are running")
	etcdPods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd").List(context.Background(), metav1.ListOptions{
		LabelSelector: "app=etcd",
	})
	if err != nil {
		return fmt.Errorf("failed to retrieve etcd pods: %v", err)
	}

	runningPods := 0
	for _, pod := range etcdPods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPods++
		}
	}

	if runningPods < 2 {
		return fmt.Errorf("expected at least 2 etcd pods running, found %d", runningPods)
	}

	g.GinkgoT().Logf("etcd cluster operator is healthy: Available=True, Degraded=False, %d pods running", runningPods)
	return nil
}

// findClusterOperatorCondition finds a condition in ClusterOperator status
func findClusterOperatorCondition(conditions []v1.ClusterOperatorStatusCondition, conditionType v1.ClusterStatusConditionType) *v1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func validateEtcdRecoveryState(
	oc *exutil.CLI, e *helpers.EtcdClientFactoryImpl,
	survivedNode, targetNode *corev1.Node,
	isTargetNodeStartedExpected, isTargetNodeLearnerExpected bool,
	timeout, pollInterval time.Duration,
) {
	o.EventuallyWithOffset(1, func() error {
		members, err := getMembers(e)
		if err != nil {
			return err
		}
		if len(members) != 2 {
			return fmt.Errorf("not enough members")
		}

		if isStarted, isLearner, err := getMemberState(survivedNode, members); err != nil {
			return err
		} else if !isStarted || isLearner {
			return fmt.Errorf("expected survived node %s to be started and voting member, got this membership instead: %+v",
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
				g.GinkgoT().Logf("Target node %s has re-started already", targetNode.Name)
			} else {
				return fmt.Errorf("expected target node %s to have status started==%v (got %v). Full membership: %+v",
					targetNode.Name, isTargetNodeStartedExpected, isStarted, members)
			}
		}
		if isTargetNodeLearnerExpected != isLearner {
			if isTargetNodeLearnerExpected && lazyCheckReboot() { // expected "learner", but "voter" already after a reboot
				g.GinkgoT().Logf("Target node %s was promoted to voter already", targetNode.Name)
			} else {
				return fmt.Errorf("expected target node %s to have status started==%v (got %v) and voting member==%v (got %v). Full membership: %+v",
					targetNode.Name, isTargetNodeStartedExpected, isStarted, isTargetNodeLearnerExpected, isLearner, members)
			}
		}

		g.GinkgoT().Logf("SUCCESS: got membership: %+v", members)
		return nil
	}, timeout, pollInterval).ShouldNot(o.HaveOccurred())
}

func validateEtcdRecoveryStateWithoutAssumingLeader(
	oc *exutil.CLI, e *helpers.EtcdClientFactoryImpl,
	nodeA, nodeB *corev1.Node,
	timeout, pollInterval time.Duration,
) (leaderNode, learnerNode *corev1.Node, learnerStarted bool) {
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
			hasNodeARebooted, err := utils.HasNodeRebooted(oc, nodeA)
			if err != nil {
				return err
			}
			hasNodeBRebooted, err := utils.HasNodeRebooted(oc, nodeB)
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
