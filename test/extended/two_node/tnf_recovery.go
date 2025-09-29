package two_node

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"slices"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	"github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	nodeIsHealthyTimeout         = time.Minute
	etcdOperatorIsHealthyTimeout = time.Minute
	memberHasLeftTimeout         = 5 * time.Minute
	memberIsLeaderTimeout        = 10 * time.Minute
	memberRejoinedLearnerTimeout = 10 * time.Minute
	memberPromotedVotingTimeout  = 15 * time.Minute
	networkDisruptionDuration    = 15 * time.Second
	pollInterval                 = 5 * time.Second
)

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Disruptive] Two Node with Fencing etcd recovery", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = util.NewCLIWithoutNamespace("").AsAdmin()
		etcdClientFactory    *helpers.EtcdClientFactoryImpl
		peerNode, targetNode corev1.Node
	)

	g.BeforeEach(func() {
		skipIfNotTopology(oc, v1.DualReplicaTopologyMode)

		g.By("Verifying etcd cluster operator is healthy before starting test")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy before starting test")

		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodes.Items)).To(o.BeNumerically("==", 2), "Expected to find 2 Nodes only")

		// Select the first index randomly
		randomIndex := rand.Intn(len(nodes.Items))
		peerNode = nodes.Items[randomIndex]
		// Select the remaining index
		targetNode = nodes.Items[(randomIndex+1)%len(nodes.Items)]

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
		err := util.TriggerNodeRebootGraceful(oc.KubeClient(), targetNode.Name)
		o.Expect(err).To(o.BeNil(), "Expected to gracefully shutdown the node without errors")
		time.Sleep(time.Minute)

		g.By(fmt.Sprintf("Ensuring %s leaves the member list", targetNode.Name))
		o.Eventually(func() error {
			return helpers.EnsureMemberRemoved(g.GinkgoT(), etcdClientFactory, targetNode.Name)
		}, memberHasLeftTimeout, pollInterval).ShouldNot(o.HaveOccurred())

		g.By(fmt.Sprintf("Ensuring that %s is a healthy voting member and adds %s back as learner", peerNode.Name, targetNode.Name))
		validateEtcdRecoveryState(etcdClientFactory,
			&survivedNode, true, false, // survivedNode expected started == true, learner == false
			&targetNode, false, true, // targetNode expected started == false, learner == true
			memberIsLeaderTimeout, pollInterval)

		g.By(fmt.Sprintf("Ensuring %s rejoins as learner", targetNode.Name))
		validateEtcdRecoveryState(etcdClientFactory,
			&survivedNode, true, false, // survivedNode expected started == true, learner == false
			&targetNode, true, true, // targetNode expected started == true, learner == true
			memberRejoinedLearnerTimeout, pollInterval)

		g.By(fmt.Sprintf("Ensuring %s node is promoted back as voting member", targetNode.Name))
		validateEtcdRecoveryState(etcdClientFactory,
			&survivedNode, true, false, // survivedNode expected started == true, learner == false
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

		g.By(fmt.Sprintf("Ensuring that %s added %s back as learner", peerNode.Name, targetNode.Name))
		validateEtcdRecoveryState(etcdClientFactory,
			&survivedNode, true, false, // survivedNode expected started == true, learner == false
			&targetNode, false, true, // targetNode expected started == false, learner == true
			memberIsLeaderTimeout, pollInterval)

		g.By(fmt.Sprintf("Ensuring %s rejoins as learner", targetNode.Name))
		validateEtcdRecoveryState(etcdClientFactory,
			&survivedNode, true, false, // survivedNode expected started == true, learner == false
			&targetNode, true, true, // targetNode expected started == true, learner == true
			memberRejoinedLearnerTimeout, pollInterval)

		g.By(fmt.Sprintf("Ensuring %s node is promoted back as voting member", targetNode.Name))
		validateEtcdRecoveryState(etcdClientFactory,
			&survivedNode, true, false, // survivedNode expected started == true, learner == false
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

		g.By("Ensuring cluster recovery with proper leader/learner roles after network disruption")
		// Note: The fenced node may recover quickly and already be started when we get
		// the first etcd membership. This is valid behavior, so we capture the learner's
		// state and adapt the test accordingly.
		leaderNode, learnerNode, learnerStarted := validateEtcdRecoveryStateWithoutAssumingLeader(etcdClientFactory,
			&peerNode, &targetNode, memberIsLeaderTimeout, pollInterval)

		if learnerStarted {
			g.GinkgoT().Printf("Learner node '%s' already started as learner\n", learnerNode.Name)
		} else {
			g.By(fmt.Sprintf("Ensuring '%s' rejoins as learner", learnerNode.Name))
			validateEtcdRecoveryState(etcdClientFactory,
				leaderNode, true, false, // survivedNode expected started == true, learner == false
				learnerNode, true, true, // targetNode expected started == true, learner == true
				memberRejoinedLearnerTimeout, pollInterval)
		}

		g.By(fmt.Sprintf("Ensuring learner node '%s' is promoted back as voting member", learnerNode.Name))
		validateEtcdRecoveryState(etcdClientFactory,
			leaderNode, true, false, // survivedNode expected started == true, learner == false
			learnerNode, true, false, // targetNode expected started == true, learner == false
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
func ensureEtcdOperatorHealthy(oc *util.CLI) error {
	g.By("Checking etcd ClusterOperator status")
	etcdOperator, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(context.Background(), "etcd", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to retrieve etcd ClusterOperator: %v", err)
	}

	// Check if etcd operator is Available
	available := findClusterOperatorCondition(etcdOperator.Status.Conditions, v1.OperatorAvailable)
	if available == nil || available.Status != v1.ConditionTrue {
		return fmt.Errorf("etcd ClusterOperator is not Available: %v", available)
	}

	// Check if etcd operator is not Degraded
	degraded := findClusterOperatorCondition(etcdOperator.Status.Conditions, v1.OperatorDegraded)
	if degraded != nil && degraded.Status == v1.ConditionTrue {
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

func validateEtcdRecoveryState(e *helpers.EtcdClientFactoryImpl,
	survivedNode *corev1.Node, isSurvivedNodeStartedExpected, isSurvivedNodeLearnerExpected bool,
	targetNode *corev1.Node, isTargetNodeStartedExpected, isTargetNodeLearnerExpected bool,
	timeout, pollInterval time.Duration) {
	o.EventuallyWithOffset(1, func() error {
		members, err := getMembers(e)
		if err != nil {
			return err
		}
		if len(members) != 2 {
			return fmt.Errorf("Not enough members")
		}

		if started, learner, err := getMemberState(survivedNode, members); err != nil {
			return err
		} else if isSurvivedNodeStartedExpected != started || isSurvivedNodeLearnerExpected != learner {
			return fmt.Errorf("Expected node: %s to be a started==%v and voting member==%v, got this membership instead: %+v",
				survivedNode.Name, isSurvivedNodeStartedExpected, isSurvivedNodeLearnerExpected, members)
		}

		// Ensure GNS node is unstarted and a learner member
		if started, learner, err := getMemberState(targetNode, members); err != nil {
			return err
		} else if isTargetNodeStartedExpected != started || isTargetNodeLearnerExpected != learner {
			return fmt.Errorf("Expected node: %s to be a started==%v and voting member==%v, got this membership instead: %+v",
				targetNode.Name, isTargetNodeStartedExpected, isTargetNodeLearnerExpected, members)
		}

		g.GinkgoT().Logf("SUCCESS: got membership: %+v", members)
		return nil
	}, timeout, pollInterval).ShouldNot(o.HaveOccurred())
}

func validateEtcdRecoveryStateWithoutAssumingLeader(e *helpers.EtcdClientFactoryImpl,
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
			return fmt.Errorf("both nodes are non-learners (should have exactly one learner): %s(started=%v, learner=%v), %s(started=%v, learner=%v)",
				nodeA.Name, startedA, learnerA, nodeB.Name, startedB, learnerB)
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
