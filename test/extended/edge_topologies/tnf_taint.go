package edge_topologies

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/edge_topologies/utils"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/services"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	taintAppliedTimeout = 5 * time.Minute
	taintRemovedTimeout = 10 * time.Minute
	journalCheckTimeout = 2 * time.Minute
)

// checkJournalOnNodes searches the systemd journal on the given nodes for a log
// entry matching the tag and pattern, returning true if found on any node.
func checkJournalOnNodes(oc *exutil.CLI, nodes []corev1.Node, tag, pattern, since string) bool {
	for _, node := range nodes {
		output, err := services.JournalGrepViaDebug(oc, node.Name, tag, pattern, since)
		if err != nil {
			framework.Logf("Warning: journal grep failed on %s: %v", node.Name, err)
			continue
		}
		if strings.TrimSpace(output) != "" {
			framework.Logf("Journal match on %s (tag=%s): %s", node.Name, tag, strings.TrimSpace(output))
			return true
		}
	}
	return false
}

// taintObserver polls both nodes for the out-of-service taint in a background
// goroutine so the test can detect transient taints that are applied and removed
// while other validations (e.g. etcd recovery) are still in progress.
type taintObserver struct {
	oc       *exutil.CLI
	nodes    []string
	interval time.Duration

	mu          sync.Mutex
	taintedNode string
	observed    bool
	stopCh      chan struct{}
}

func newTaintObserver(oc *exutil.CLI, nodes []string, interval time.Duration) *taintObserver {
	return &taintObserver{
		oc:       oc,
		nodes:    nodes,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (t *taintObserver) Start() {
	go func() {
		for {
			select {
			case <-t.stopCh:
				return
			default:
				for _, name := range t.nodes {
					node, err := services.FetchNodeObject(t.oc, name)
					if err != nil {
						continue
					}
					if services.HasOutOfServiceTaint(node) {
						t.mu.Lock()
						if !t.observed {
							framework.Logf("taintObserver: detected out-of-service taint on %s", name)
						}
						t.taintedNode = name
						t.observed = true
						t.mu.Unlock()
					}
				}
				time.Sleep(t.interval)
			}
		}
	}()
}

func (t *taintObserver) Stop() {
	close(t.stopCh)
}

func (t *taintObserver) WasTaintObserved() (nodeName string, observed bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.taintedNode, t.observed
}

var _ = g.Describe("[sig-node][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node] Two Node with Fencing taint safety", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLIWithoutNamespace("").AsAdmin()
	)

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)
	})

	g.It("should have pacemaker taint and untaint alerts registered", func() {
		nodes, err := utils.GetNodes(oc, utils.LabelNodeRoleControlPlane)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve control-plane nodes")
		o.Expect(nodes.Items).NotTo(o.BeEmpty(), "Expected at least one control-plane node")

		execNode := nodes.Items[0]

		g.By("Checking pacemaker alert configuration")
		alertOutput, err := services.PcsAlertConfigViaDebug(oc, execNode.Name)
		o.Expect(err).ToNot(o.HaveOccurred(), "Expected pcs alert config to succeed")

		o.Expect(alertOutput).To(o.ContainSubstring(services.TaintAlertID),
			fmt.Sprintf("Expected pacemaker alert %s to be registered", services.TaintAlertID))
		o.Expect(alertOutput).To(o.ContainSubstring(services.UntaintAlertID),
			fmt.Sprintf("Expected pacemaker alert %s to be registered", services.UntaintAlertID))
		framework.Logf("Pacemaker alert config:\n%s", alertOutput)

		g.By("Verifying alert scripts exist on disk")
		for _, script := range []string{services.TaintAlertScriptPath, services.UntaintAlertScriptPath} {
			output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, execNode.Name, "default",
				"bash", "-c", fmt.Sprintf("test -x %s && echo EXISTS || echo MISSING", script))
			o.Expect(err).ToNot(o.HaveOccurred(),
				fmt.Sprintf("Expected to check existence of %s", script))
			o.Expect(strings.TrimSpace(output)).To(o.Equal("EXISTS"),
				fmt.Sprintf("Expected %s to exist and be executable", script))
		}
	})
})

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Serial][Disruptive] Two Node with Fencing taint lifecycle", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = exutil.NewCLIWithoutNamespace("").AsAdmin()
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

		// Safety net: remove any lingering taint/annotation that could break
		// subsequent tests if this test fails mid-way.
		g.DeferCleanup(func() {
			services.RemoveTaintAndAnnotation(oc, peerNode.Name)
			services.RemoveTaintAndAnnotation(oc, targetNode.Name)
		})

		// LIFO: registered last so it runs first, capturing state before cleanup.
		g.DeferCleanup(func() {
			logFinalClusterStatus([]corev1.Node{peerNode, targetNode})
		})
	})

	g.It("should apply and remove out-of-service taint and annotation during network disruption recovery", func() {
		g.By("Recording timestamp before disruption for journal log scoping")
		baseTimestamp, err := services.GetTimestampViaDebug(oc, peerNode.Name)
		o.Expect(err).ToNot(o.HaveOccurred(), "Expected to capture baseline timestamp")
		framework.Logf("Baseline timestamp: %s", baseTimestamp)

		g.By("Starting background taint observer on both nodes")
		observer := newTaintObserver(oc, []string{peerNode.Name, targetNode.Name}, 2*time.Second)
		observer.Start()
		defer observer.Stop()

		g.By(fmt.Sprintf("Blocking network communication between %s and %s for %v",
			targetNode.Name, peerNode.Name, networkDisruptionDuration))
		command, err := exutil.TriggerNetworkDisruption(oc.KubeClient(), &targetNode, &peerNode, networkDisruptionDuration)
		o.Expect(err).To(o.BeNil(), "Expected to disrupt network without errors")
		framework.Logf("Network disruption command: %s", command)

		g.By(fmt.Sprintf("Ensuring cluster recovery with proper leader/learner roles (timeout: %v)", memberIsLeaderTimeout))
		leaderNode, learnerNode, learnerStarted := validateEtcdRecoveryStateWithoutAssumingLeader(
			oc, etcdClientFactory, &peerNode, &targetNode, memberIsLeaderTimeout, utils.FiveSecondPollInterval)
		framework.Logf("Leader: %s, Learner (fenced): %s, learner already started: %v",
			leaderNode.Name, learnerNode.Name, learnerStarted)

		// --- Taint Application Checks ---

		// Determine which node was actually fenced by pacemaker. The fenced
		// node is not necessarily the etcd learner - pacemaker makes an
		// independent fencing decision. We check the background observer
		// first (catches transient taints), then poll both nodes live.
		var fencedNode, survivedNode *corev1.Node

		observedNode, taintSeen := observer.WasTaintObserved()
		if taintSeen {
			framework.Logf("Out-of-service taint was observed on %s by background observer", observedNode)
		}

		// Check both nodes for a current taint
		for _, candidate := range []*corev1.Node{leaderNode, learnerNode} {
			n, fetchErr := services.FetchNodeObject(oc, candidate.Name)
			o.Expect(fetchErr).ToNot(o.HaveOccurred())
			if services.HasOutOfServiceTaint(n) {
				framework.Logf("Out-of-service taint is currently present on %s", candidate.Name)
				fencedNode = candidate
				break
			}
		}

		// If no taint found yet (not observed, not currently present), wait for it on either node
		if fencedNode == nil && !taintSeen {
			g.By(fmt.Sprintf("Waiting for out-of-service taint to appear on either node (timeout: %v)",
				taintAppliedTimeout))
			o.Eventually(func() bool {
				name, seen := observer.WasTaintObserved()
				if seen {
					observedNode = name
					return true
				}
				for _, candidate := range []*corev1.Node{leaderNode, learnerNode} {
					n, fetchErr := services.FetchNodeObject(oc, candidate.Name)
					if fetchErr != nil {
						framework.Logf("Waiting for taint: could not fetch node %s: %v", candidate.Name, fetchErr)
						continue
					}
					if services.HasOutOfServiceTaint(n) {
						return true
					}
				}
				return false
			}, taintAppliedTimeout, utils.FiveSecondPollInterval).Should(o.BeTrue(),
				"Out-of-service taint should appear on exactly one node")
		}

		// Resolve fencedNode from observer if we only saw it transiently
		if fencedNode == nil && observedNode == "" {
			observedNode, _ = observer.WasTaintObserved()
		}
		if fencedNode == nil {
			o.Expect(observedNode).ToNot(o.BeEmpty(), "Should have identified the fenced node via taint observation")
			if observedNode == leaderNode.Name {
				fencedNode = leaderNode
			} else {
				fencedNode = learnerNode
			}
		}

		if fencedNode.Name == leaderNode.Name {
			survivedNode = learnerNode
		} else {
			survivedNode = leaderNode
		}
		framework.Logf("Fenced node: %s, Survived node: %s", fencedNode.Name, survivedNode.Name)

		g.By(fmt.Sprintf("Verifying survived node %s is NOT tainted", survivedNode.Name))
		survivedRefresh, err := services.FetchNodeObject(oc, survivedNode.Name)
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(services.HasOutOfServiceTaint(survivedRefresh)).To(o.BeFalse(),
			fmt.Sprintf("Survived node %s should NOT have out-of-service taint", survivedNode.Name))
		o.Expect(services.HasOutOfServiceAnnotation(survivedRefresh)).To(o.BeFalse(),
			fmt.Sprintf("Survived node %s should NOT have out-of-service annotation", survivedNode.Name))

		g.By("Verifying taint alert journal log on survived node")
		o.Eventually(func() bool {
			return checkJournalOnNodes(oc, []corev1.Node{*survivedNode},
				services.TaintAlertLogTag, services.TaintAlertFencingLog, baseTimestamp)
		}, journalCheckTimeout, utils.FiveSecondPollInterval).Should(o.BeTrue(),
			"tnf-taint-alert should log fencing success on survived node")

		g.By("Verifying taint script journal log on survived node")
		o.Eventually(func() bool {
			return checkJournalOnNodes(oc, []corev1.Node{*survivedNode},
				services.TaintScriptLogTag, services.TaintSuccessLog, baseTimestamp)
		}, journalCheckTimeout, utils.FiveSecondPollInterval).Should(o.BeTrue(),
			"taint-fenced-node should log successful taint application")

		taintUnit := fmt.Sprintf(services.TaintServiceUnitFmt, fencedNode.Name)
		g.By(fmt.Sprintf("Verifying taint systemd service journal (%s) shows completion", taintUnit))
		o.Eventually(func() bool {
			output, err := services.SystemdServiceJournalGrep(oc, survivedNode.Name, taintUnit,
				"Finished Taint fenced node", baseTimestamp)
			if err != nil {
				return false
			}
			return strings.TrimSpace(output) != ""
		}, journalCheckTimeout, utils.FiveSecondPollInterval).Should(o.BeTrue(),
			fmt.Sprintf("systemd journal for %s should show service completion", taintUnit))

		// --- Recovery Wait ---

		if !learnerStarted {
			g.By(fmt.Sprintf("Ensuring %s rejoins as learner (timeout: %v)",
				learnerNode.Name, memberRejoinedLearnerTimeout))
			validateEtcdRecoveryState(oc, etcdClientFactory,
				leaderNode,
				learnerNode, true, true,
				memberRejoinedLearnerTimeout, utils.FiveSecondPollInterval)
		}

		g.By(fmt.Sprintf("Ensuring %s is promoted back as voting member (timeout: %v)",
			learnerNode.Name, memberPromotedVotingTimeout))
		validateEtcdRecoveryState(oc, etcdClientFactory,
			leaderNode,
			learnerNode, true, false,
			memberPromotedVotingTimeout, utils.FiveSecondPollInterval)

		// --- Taint Removal Checks ---

		g.By(fmt.Sprintf("Verifying out-of-service taint is removed from fenced node %s after recovery (timeout: %v)",
			fencedNode.Name, taintRemovedTimeout))
		o.Eventually(func() bool {
			node, err := services.FetchNodeObject(oc, fencedNode.Name)
			if err != nil {
				framework.Logf("Waiting for untaint: could not fetch node %s: %v", fencedNode.Name, err)
				return false
			}
			return !services.HasOutOfServiceTaint(node)
		}, taintRemovedTimeout, utils.FiveSecondPollInterval).Should(o.BeTrue(),
			fmt.Sprintf("Taint should be removed from fenced node %s after recovery", fencedNode.Name))

		g.By(fmt.Sprintf("Verifying out-of-service annotation is removed from fenced node %s after recovery", fencedNode.Name))
		fencedRefresh, err := services.FetchNodeObject(oc, fencedNode.Name)
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(services.HasOutOfServiceAnnotation(fencedRefresh)).To(o.BeFalse(),
			fmt.Sprintf("Annotation should be removed from fenced node %s after recovery", fencedNode.Name))

		g.By("Verifying untaint alert journal log")
		bothNodes := []corev1.Node{*survivedNode, *fencedNode}
		o.Eventually(func() bool {
			return checkJournalOnNodes(oc, bothNodes,
				services.UntaintAlertLogTag, services.UntaintAlertRejoinLog, baseTimestamp)
		}, journalCheckTimeout, utils.FiveSecondPollInterval).Should(o.BeTrue(),
			"tnf-untaint-alert should log node rejoin event on at least one node")

		g.By("Verifying untaint script journal log")
		o.Eventually(func() bool {
			return checkJournalOnNodes(oc, bothNodes,
				services.UntaintScriptLogTag, services.UntaintSuccessLog, baseTimestamp)
		}, taintRemovedTimeout, utils.FiveSecondPollInterval).Should(o.BeTrue(),
			"untaint-fenced-node should log successful untaint on at least one node")

		untaintUnit := fmt.Sprintf(services.UntaintServiceUnitFmt, fencedNode.Name)
		g.By(fmt.Sprintf("Verifying untaint systemd service journal (%s) shows completion", untaintUnit))
		o.Eventually(func() bool {
			for _, n := range bothNodes {
				output, err := services.SystemdServiceJournalGrep(oc, n.Name, untaintUnit,
					"Finished Untaint pacemaker-annotated nodes", baseTimestamp)
				if err != nil {
					continue
				}
				if strings.TrimSpace(output) != "" {
					framework.Logf("Systemd journal for %s on %s: %s", untaintUnit, n.Name, strings.TrimSpace(output))
					return true
				}
			}
			return false
		}, journalCheckTimeout, utils.FiveSecondPollInterval).Should(o.BeTrue(),
			fmt.Sprintf("systemd journal for %s should show service completion on at least one node", untaintUnit))
	})
})
