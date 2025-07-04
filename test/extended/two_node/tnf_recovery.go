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
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	nodeIsHealthyTimeout         = time.Minute
	memberHasLeftTimeout         = 5 * time.Minute
	memberIsLeaderTimeout        = 2 * time.Minute
	memberRejoinedLearnerTimeout = 10 * time.Minute
	memberPromotedVotingTimeout  = 10 * time.Minute
	pollInterval                 = 5 * time.Second
)

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Disruptive] Two Node with Fencing etcd recovery", func() {
	defer g.GinkgoRecover()

	var (
		oc                = exutil.NewCLIWithoutNamespace("").AsAdmin()
		etcdClientFactory *helpers.EtcdClientFactoryImpl
		nodeA, nodeB      corev1.Node
	)

	g.BeforeEach(func() {
		skipIfNotTopology(oc, v1.DualReplicaTopologyMode)

		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		o.Expect(len(nodes.Items)).To(o.BeNumerically("==", 2), "Expected to find 2 Nodes only")

		// Select the first index randomly
		randomIndex := rand.Intn(len(nodes.Items))
		nodeA = nodes.Items[randomIndex]
		// Select the remaining index
		nodeB = nodes.Items[(randomIndex+1)%len(nodes.Items)]
		g.GinkgoT().Printf("Randomly selected %s (%s) to be gracefully shut down and %s (%s) to take the lead\n", nodeB.Name, nodeB.Status.Addresses[0].Address, nodeA.Name, nodeA.Status.Addresses[0].Address)

		kubeClient := oc.KubeClient()
		etcdClientFactory = helpers.NewEtcdClientFactory(kubeClient)

		g.GinkgoT().Printf("Ensure both nodes are healthy before starting the test\n")
		o.Eventually(func() error {
			return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, nodeA.Name)
		}, nodeIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "expect to ensure Node A healthy without error")

		o.Eventually(func() error {
			return helpers.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, nodeB.Name)
		}, nodeIsHealthyTimeout, pollInterval).ShouldNot(o.HaveOccurred(), "expect to ensure Node B healthy without error")
	})

	g.It("Should support a graceful node shutdown", func() {
		msg := fmt.Sprintf("Shutting down %s gracefully in 1 minute", nodeB.Name)
		g.By(msg)
		// NOTE: Using `shutdown` alone would cause the node to be permanently removed from the cluster.
		// To prevent this, we use the `--reboot` flag, which ensures a graceful shutdown and allows the
		// node to rejoin the cluster upon restart. A one-minute delay is added to give the debug node
		// sufficient time to cleanly exit before the shutdown process completes.
		_, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeB.Name, "openshift-etcd", "shutdown", "--reboot", "+1")
		o.Expect(err).To(o.BeNil(), "Expected to gracefully shutdown the node without errors")
		time.Sleep(time.Minute)

		msg = fmt.Sprintf("Ensuring %s leaves the member list", nodeB.Name)
		g.By(msg)
		o.Eventually(func() error {
			return helpers.EnsureMemberRemoved(g.GinkgoT(), etcdClientFactory, nodeB.Name)
		}, memberHasLeftTimeout, pollInterval).ShouldNot(o.HaveOccurred())

		msg = fmt.Sprintf("Ensuring that %s is a healthy voting member and adds %s back as learner", nodeA.Name, nodeB.Name)
		g.By(msg)
		o.Eventually(func() error {
			members, err := getMembers(etcdClientFactory)
			if err != nil {
				return err
			}
			if len(members) != 2 {
				return fmt.Errorf("Not enough members")
			}

			if started, learner, err := getMemberState(&nodeA, members); err != nil {
				return err
			} else if !started || learner {
				return fmt.Errorf("Expected node: %s to be a started and voting member. Membership: %+v", nodeA.Name, members)
			}

			// Ensure GNS node is unstarted and a learner member
			if started, learner, err := getMemberState(&nodeB, members); err != nil {
				return err
			} else if started || !learner {
				return fmt.Errorf("Expected node: %s to be a unstarted and learning member. Membership: %+v", nodeB.Name, members)
			}

			g.GinkgoT().Logf("membership: %+v", members)
			return nil
		}, memberIsLeaderTimeout, pollInterval).ShouldNot(o.HaveOccurred())

		msg = fmt.Sprintf("Ensuring %s rejoins as learner", nodeB.Name)
		g.By(msg)
		o.Eventually(func() error {
			members, err := getMembers(etcdClientFactory)
			if err != nil {
				return err
			}
			if len(members) != 2 {
				return fmt.Errorf("Not enough members")
			}

			if started, learner, err := getMemberState(&nodeA, members); err != nil {
				return err
			} else if !started || learner {
				return fmt.Errorf("Expected node: %s to be a started and voting member. Membership: %+v", nodeA.Name, members)
			}

			if started, learner, err := getMemberState(&nodeB, members); err != nil {
				return err
			} else if !started || !learner {
				return fmt.Errorf("Expected node: %s to be a started and learner member. Membership: %+v", nodeB.Name, members)
			}

			g.GinkgoT().Logf("membership: %+v", members)
			return nil
		}, memberRejoinedLearnerTimeout, pollInterval).ShouldNot(o.HaveOccurred())

		msg = fmt.Sprintf("Ensuring %s node is promoted back as voting member", nodeB.Name)
		g.By(msg)
		o.Eventually(func() error {
			members, err := getMembers(etcdClientFactory)
			if err != nil {
				return err
			}
			if len(members) != 2 {
				return fmt.Errorf("Not enough members")
			}

			if started, learner, err := getMemberState(&nodeA, members); err != nil {
				return err
			} else if !started || learner {
				return fmt.Errorf("Expected node: %s to be a started and voting member. Membership: %+v", nodeA.Name, members)
			}

			if started, learner, err := getMemberState(&nodeB, members); err != nil {
				return err
			} else if !started || learner {
				return fmt.Errorf("Expected node: %s to be a started and voting member. Membership: %+v", nodeB.Name, members)
			}

			g.GinkgoT().Logf("membership: %+v", members)
			return nil
		}, memberPromotedVotingTimeout, pollInterval).ShouldNot(o.HaveOccurred())
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
