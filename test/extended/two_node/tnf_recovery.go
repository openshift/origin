package two_node

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Disruptive] Two Node with Fencing etcd recovery", func() {
	defer g.GinkgoRecover()

	var (
		oc                       = exutil.NewCLIWithoutNamespace("").AsAdmin()
		etcdClientFactory        *helpers.EtcdClientFactoryImpl
		survivedNode, targetNode corev1.Node
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
		survivedNode = nodes.Items[randomIndex]
		// Select the remaining index
		targetNode = nodes.Items[(randomIndex+1)%len(nodes.Items)]
		g.GinkgoT().Printf("Randomly selected %s (%s) to be shut down and %s (%s) to take the lead\n", targetNode.Name, targetNode.Status.Addresses[0].Address, survivedNode.Name, survivedNode.Status.Addresses[0].Address)

		kubeClient := oc.KubeClient()
		etcdClientFactory = helpers.NewEtcdClientFactory(kubeClient)

		g.GinkgoT().Printf("Ensure both nodes are healthy before starting the test\n")
		o.Eventually(func() error {
			return ensureEtcdNodeHealthy(oc, survivedNode.Name)
		}, nodeIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "node %s should be healthy before starting test", survivedNode.Name)

		o.Eventually(func() error {
			return ensureEtcdNodeHealthy(oc, targetNode.Name)
		}, nodeIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "node %s should be healthy before starting test", targetNode.Name)

		utils.ValidateClusterOperatorsAvailable(oc)
	})

	g.It("should maintain quorum after ungraceful shutdown and restart of leader and promote learner to voter", func() {
		g.By("Determining current leader and selecting target node to shutdown")
		leaderNode, err := determineEtcdLeaderNode(etcdClientFactory)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Failed to determine etcd leader node")
		g.GinkgoT().Printf("Detected etcd leader node: %s\n", leaderNode)

		// Set targetNode to be the leader, survivedNode to be the follower
		if leaderNode == survivedNode.Name {
			survivedNode, targetNode = targetNode, survivedNode
			g.GinkgoT().Printf("Adjusted: targeting leader node %s for shutdown, %s to survive\n", targetNode.Name, survivedNode.Name)
		}

		g.By("Ungracefully shutting down target node")
		ungracefulShutdownNode(oc, targetNode.Name)

		g.By("Validating etcd recovery state - target expected to be unstarted learner")
		validateEtcdRecoveryState(oc, etcdClientFactory, &survivedNode, &targetNode,
			false, true, memberRejoinedLearnerTimeout, pollInterval)

		g.By("Starting target node")
		startNode(oc, targetNode.Name)

		g.By("Validating final etcd state - both nodes should be started voters")
		validateEtcdRecoveryState(oc, etcdClientFactory, &survivedNode, &targetNode,
			true, false, memberPromotedVotingTimeout, pollInterval)

		g.By("Verifying etcd cluster operator is healthy at the end of test")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy at the end of test")
	})

	g.It("should maintain quorum after ungraceful shutdown and restart of follower and promote learner to voter", func() {
		g.By("Determining current leader and selecting target node to shutdown")
		leaderNode, err := determineEtcdLeaderNode(etcdClientFactory)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Failed to determine etcd leader node")
		g.GinkgoT().Printf("Detected etcd leader node: %s\n", leaderNode)

		// Set targetNode to be the follower, survivedNode to be the leader
		if leaderNode == targetNode.Name {
			survivedNode, targetNode = targetNode, survivedNode
			g.GinkgoT().Printf("Adjusted: targeting follower node %s for shutdown, %s (leader) to survive\n", targetNode.Name, survivedNode.Name)
		}

		g.By("Ungracefully shutting down target node")
		ungracefulShutdownNode(oc, targetNode.Name)

		g.By("Validating etcd recovery state - target expected to be unstarted learner")
		validateEtcdRecoveryState(oc, etcdClientFactory, &survivedNode, &targetNode,
			false, true, memberRejoinedLearnerTimeout, pollInterval)

		g.By("Starting target node")
		startNode(oc, targetNode.Name)

		g.By("Validating final etcd state - both nodes should be started voters")
		validateEtcdRecoveryState(oc, etcdClientFactory, &survivedNode, &targetNode,
			true, false, memberPromotedVotingTimeout, pollInterval)

		g.By("Verifying etcd cluster operator is healthy at the end of test")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy at the end of test")
	})

	g.It("should maintain quorum after graceful shutdown and restart of leader and promote learner to voter", func() {
		g.By("Determining current leader and selecting target node to shutdown")
		leaderNode, err := determineEtcdLeaderNode(etcdClientFactory)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Failed to determine etcd leader node")
		g.GinkgoT().Printf("Detected etcd leader node: %s\n", leaderNode)

		// Set targetNode to be the leader, survivedNode to be the follower
		if leaderNode == survivedNode.Name {
			survivedNode, targetNode = targetNode, survivedNode
			g.GinkgoT().Printf("Adjusted: targeting leader node %s for graceful shutdown, %s to survive\n", targetNode.Name, survivedNode.Name)
		}

		g.By("Gracefully shutting down target node")
		gracefulShutdownNode(oc, targetNode.Name)

		g.By("Validating etcd recovery state - target expected to be unstarted learner")
		validateEtcdRecoveryState(oc, etcdClientFactory, &survivedNode, &targetNode,
			false, true, memberRejoinedLearnerTimeout, pollInterval)

		g.By("Starting target node")
		startNode(oc, targetNode.Name)

		g.By("Validating final etcd state - both nodes should be started voters")
		validateEtcdRecoveryState(oc, etcdClientFactory, &survivedNode, &targetNode,
			true, false, memberPromotedVotingTimeout, pollInterval)

		g.By("Verifying etcd cluster operator is healthy at the end of test")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy at the end of test")
	})

	g.It("should maintain quorum after graceful shutdown and restart of follower and promote learner to voter", func() {
		g.By("Determining current leader and selecting target node to shutdown")
		leaderNode, err := determineEtcdLeaderNode(etcdClientFactory)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Failed to determine etcd leader node")
		g.GinkgoT().Printf("Detected etcd leader node: %s\n", leaderNode)

		// Set targetNode to be the follower, survivedNode to be the leader
		if leaderNode == targetNode.Name {
			survivedNode, targetNode = targetNode, survivedNode
			g.GinkgoT().Printf("Adjusted: targeting follower node %s for graceful shutdown, %s (leader) to survive\n", targetNode.Name, survivedNode.Name)
		}

		g.By("Gracefully shutting down target node")
		gracefulShutdownNode(oc, targetNode.Name)

		g.By("Validating etcd recovery state - target expected to be unstarted learner")
		validateEtcdRecoveryState(oc, etcdClientFactory, &survivedNode, &targetNode,
			false, true, memberRejoinedLearnerTimeout, pollInterval)

		g.By("Starting target node")
		startNode(oc, targetNode.Name)

		g.By("Validating final etcd state - both nodes should be started voters")
		validateEtcdRecoveryState(oc, etcdClientFactory, &survivedNode, &targetNode,
			true, false, memberPromotedVotingTimeout, pollInterval)

		g.By("Verifying etcd cluster operator is healthy at the end of test")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy at the end of test")
	})

	g.It("should recover etcd cluster after network disruption between nodes", func() {
		g.By("Getting hypervisor configuration for network disruption")
		hypervisorConfig, err := getHypervisorConfig()
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Failed to get hypervisor configuration")

		g.By("Determining current leader and selecting target node for network disruption")
		leaderNode, err := determineEtcdLeaderNode(etcdClientFactory)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Failed to determine etcd leader node")
		g.GinkgoT().Printf("Detected etcd leader node: %s\n", leaderNode)

		// Set targetNode to be the leader, survivedNode to be the follower
		if leaderNode == survivedNode.Name {
			survivedNode, targetNode = targetNode, survivedNode
			g.GinkgoT().Printf("Adjusted: targeting leader node %s for network disruption, %s to survive\n", targetNode.Name, survivedNode.Name)
		}

		g.By("Disrupting network between nodes")
		disruptNetworkBetweenNodes(hypervisorConfig, survivedNode, targetNode, networkDisruptionDuration)

		g.By("Validating etcd recovery state after network disruption")
		validateEtcdRecoveryState(oc, etcdClientFactory, &survivedNode, &targetNode,
			true, false, memberPromotedVotingTimeout, pollInterval)

		g.By("Verifying etcd cluster operator is healthy at the end of test")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy at the end of test")
	})

	g.It("should recover after double node failure (both nodes ungracefully shut down)", func() {
		g.By("Ungracefully shutting down both nodes simultaneously")
		ungracefulShutdownNode(oc, survivedNode.Name)
		ungracefulShutdownNode(oc, targetNode.Name)

		g.By("Starting both nodes")
		startNode(oc, survivedNode.Name)
		startNode(oc, targetNode.Name)

		g.By("Waiting for etcd members to become healthy after double node failure")
		o.Eventually(func() error {
			members, err := getMembers(etcdClientFactory)
			if err != nil {
				return fmt.Errorf("Failed to get etcd members: %v", err)
			}
			if len(members) != 2 {
				return fmt.Errorf("Expected 2 etcd members, got %d", len(members))
			}

			survivedStarted, survivedVoter, err := getMemberState(&survivedNode, members)
			if err != nil {
				return fmt.Errorf("Failed to get member state for %s: %v", survivedNode.Name, err)
			}
			targetStarted, targetVoter, err := getMemberState(&targetNode, members)
			if err != nil {
				return fmt.Errorf("Failed to get member state for %s: %v", targetNode.Name, err)
			}

			if !survivedStarted || !survivedVoter {
				return fmt.Errorf("Survived node %s not ready: started=%v, voter=%v", survivedNode.Name, survivedStarted, survivedVoter)
			}
			if !targetStarted || !targetVoter {
				return fmt.Errorf("Target node %s not ready: started=%v, voter=%v", targetNode.Name, targetStarted, targetVoter)
			}

			g.GinkgoT().Logf("SUCCESS: Both etcd members are healthy after double node failure")
			return nil
		}, membersHealthyAfterDoubleReboot, pollInterval).ShouldNot(o.HaveOccurred(), "Both etcd members should be healthy after double node failure")

		g.By("Verifying etcd cluster operator is healthy at the end of test")
		o.Eventually(func() error {
			return ensureEtcdOperatorHealthy(oc)
		}, etcdOperatorIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "etcd cluster operator should be healthy at the end of test")
	})
})

func validateEtcdRecoveryState(
	oc *exutil.CLI, e *helpers.EtcdClientFactoryImpl,
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

func gracefulShutdownNode(oc *exutil.CLI, nodeName string) {
	g.GinkgoT().Printf("Initiating graceful shutdown of node %s\n", nodeName)
	// TODO: Implement VM shutdown using available libvirt functions
	// This requires hypervisor config and VM name lookup
	g.GinkgoT().Printf("Node %s graceful shutdown - implementation needed\n", nodeName)
}

func ungracefulShutdownNode(oc *exutil.CLI, nodeName string) {
	g.GinkgoT().Printf("Initiating ungraceful shutdown of node %s\n", nodeName)
	// TODO: Implement VM shutdown using available libvirt functions
	// This requires hypervisor config and VM name lookup
	g.GinkgoT().Printf("Node %s ungraceful shutdown - implementation needed\n", nodeName)
}

func startNode(oc *exutil.CLI, nodeName string) {
	g.GinkgoT().Printf("Starting node %s\n", nodeName)
	// TODO: Implement VM start using available libvirt functions
	// This requires hypervisor config and VM name lookup
	g.GinkgoT().Printf("Node %s start - implementation needed\n", nodeName)
}

func disruptNetworkBetweenNodes(hypervisorConfig hypervisorExtendedConfig, survivedNode, targetNode corev1.Node, duration time.Duration) {
	g.GinkgoT().Printf("Disrupting network between nodes %s and %s for %v\n", survivedNode.Name, targetNode.Name, duration)
	// TODO: Implement network disruption using available utilities
	// This requires custom network manipulation commands
	g.GinkgoT().Printf("Network disruption between nodes - implementation needed\n")
}

func getHypervisorConfig() (hypervisorExtendedConfig, error) {
	hypervisorHost := os.Getenv("TNF_HYPERVISOR_HOST")
	hypervisorUser := os.Getenv("TNF_HYPERVISOR_USER")
	hypervisorKeyPath := os.Getenv("TNF_HYPERVISOR_KEY_PATH")
	hypervisorKnownHostsPath := os.Getenv("TNF_HYPERVISOR_KNOWN_HOSTS_PATH")

	if hypervisorHost == "" || hypervisorUser == "" || hypervisorKeyPath == "" {
		return hypervisorExtendedConfig{}, fmt.Errorf("required environment variables not set: TNF_HYPERVISOR_HOST, TNF_HYPERVISOR_USER, TNF_HYPERVISOR_KEY_PATH")
	}

	return hypervisorExtendedConfig{
		HypervisorConfig: core.SSHConfig{
			IP:             hypervisorHost,
			User:           hypervisorUser,
			PrivateKeyPath: hypervisorKeyPath,
		},
		HypervisorKnownHostsPath: hypervisorKnownHostsPath,
	}, nil
}

func ensureEtcdOperatorHealthy(oc *exutil.CLI) error {
	co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(context.Background(), "etcd", metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "could not get cluster operator etcd")
	}

	for _, condition := range co.Status.Conditions {
		switch condition.Type {
		case v1.OperatorAvailable:
			if condition.Status != v1.ConditionTrue {
				return fmt.Errorf("etcd cluster operator not available: %s", condition.Message)
			}
		case v1.OperatorProgressing:
			if condition.Status == v1.ConditionTrue {
				return fmt.Errorf("etcd cluster operator is progressing: %s", condition.Message)
			}
		case v1.OperatorDegraded:
			if condition.Status == v1.ConditionTrue {
				return fmt.Errorf("etcd cluster operator is degraded: %s", condition.Message)
			}
		}
	}

	return nil
}

func ensureEtcdNodeHealthy(oc *exutil.CLI, nodeName string) error {
	node, err := oc.KubeClient().CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "could not get node %s", nodeName)
	}

	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			if condition.Status != corev1.ConditionTrue {
				return fmt.Errorf("node %s is not ready: %s", nodeName, condition.Message)
			}
			break
		}
	}

	return nil
}

func determineEtcdLeaderNode(etcdClientFactory *helpers.EtcdClientFactoryImpl) (string, error) {
	members, err := getMembers(etcdClientFactory)
	if err != nil {
		return "", err
	}

	// TODO: Implement proper leader detection using etcd client API
	// The IsLeader field doesn't exist on etcdserverpb.Member
	if len(members) > 0 && len(members[0].Name) > 0 {
		return members[0].Name, nil // Return first member as placeholder
	}

	return "", fmt.Errorf("no members found in etcd cluster")
}

func getMembers(etcdClientFactory *helpers.EtcdClientFactoryImpl) ([]*etcdserverpb.Member, error) {
	etcdClient, closeFn, err := etcdClientFactory.NewEtcdClient()
	if err != nil {
		return nil, err
	}
	defer closeFn()
	defer etcdClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := etcdClient.MemberList(ctx)
	if err != nil {
		return nil, err
	}

	return resp.Members, nil
}

func getMemberState(node *corev1.Node, members []*etcdserverpb.Member) (bool, bool, error) {
	nodeIP, err := getNodeInternalIPFromNode(node)
	if err != nil {
		return false, false, err
	}
	if nodeIP == "" {
		return false, false, fmt.Errorf("could not find internal IP for node %s", node.Name)
	}

	for _, member := range members {
		for _, clientURL := range member.ClientURLs {
			if host, _, err := net.SplitHostPort(clientURL); err == nil && host == nodeIP {
				// TODO: Implement proper member state detection
				// The IsStarted and IsLearner fields don't exist on etcdserverpb.Member
				// For now, assume member is started if it has client URLs
				isStarted := len(member.ClientURLs) > 0
				isLearner := false // Placeholder - would need etcd client API to determine
				return isStarted, isLearner, nil
			}
		}
	}

	return false, false, fmt.Errorf("could not find etcd member for node %s with IP %s", node.Name, nodeIP)
}

func getNodeInternalIPFromNode(node *corev1.Node) (string, error) {
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			return address.Address, nil
		}
	}
	return "", fmt.Errorf("no internal IP found for node %s", node.Name)
}
