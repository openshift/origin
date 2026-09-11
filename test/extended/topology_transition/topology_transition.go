package topology_transition

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	etcdhelpers "github.com/openshift/origin/test/extended/etcd/helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
	coutil "github.com/openshift/origin/test/extended/util/operator"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	requiredControlPlaneNodes = 3
	requiredEtcdVotingMembers = 3

	// The CI lane is expected to have already brought the cluster to the
	// preconditions-satisfied state before this suite runs, so this is a
	// bounded sanity wait, not the lane's own (much longer) provisioning
	// budget.
	preconditionWaitTimeout = 20 * time.Minute

	// clusterOperatorStabilityTimeout bounds a pre-transition check that
	// cluster operators are stable, mirroring one of the controller's own
	// preflight checks (validateClusterOperatorsStable). This is a defensive,
	// fast-fail check for test clarity: without it, operators left unstable
	// by an earlier failure would surface as a confusing PreflightCheckFailed
	// on the transition-admission assertion below instead of a clear failure
	// here.
	clusterOperatorStabilityTimeout = 5 * time.Minute

	// admissionWaitTimeout and statusConvergeTimeout are both short: the
	// controller sets Progressing=True/Upgradeable=False and updates
	// status.controlPlaneTopology/infrastructureTopology in the same sync()
	// call on admission -- neither is gated by the controller's
	// reconciliation soak (see completionWaitTimeout below). The timeouts
	// are a generous ceiling on top of that near-immediate write, not a
	// re-implementation of it.
	admissionWaitTimeout  = 5 * time.Minute
	statusConvergeTimeout = 5 * time.Minute

	// completionWaitTimeout must cover both the controller's own ~5 minute
	// reconciliation soak (minReconciliationSoakTime) AND the real rollout
	// time for everything checkClusterReconciliation waits on afterwards:
	// new rendered master/worker MachineConfigs, the master MachineConfigPool
	// rolling out to all 3 nodes, kube-apiserver/openshift-apiserver
	// reconciling to the new node count, and ingress replicas -- a real
	// rollout across a 3-node control plane can plausibly take well beyond
	// the 5 minute soak floor.
	completionWaitTimeout = 45 * time.Minute
	operatorSettleTimeout = 20 * time.Minute

	// nodeInformerPropagationWait is a buffer between cordoning a node and
	// requesting a transition in the negative test below. The controller's
	// node informer and infrastructure informer are independent watch
	// streams; without this buffer, a spec-change watch event could in
	// principle reach the controller and trigger preflight evaluation before
	// the cordon's watch event has updated its node lister, letting a
	// negative test that is meant to be non-destructive instead admit a real
	// transition. A fixed sleep only shrinks this window rather than closing
	// it deterministically (the controller's informer state isn't externally
	// observable from a black-box e2e test), but it makes the race
	// negligibly unlikely: both watch events are typically delivered in well
	// under this window.
	nodeInformerPropagationWait = 15 * time.Second

	baselineWorkloadName = "topology-transition-baseline"
)

// This suite triggers and validates a SNO -> HA compact (3-node) control-plane
// topology transition on platform:none, gated behind the MutableTopology
// feature gate. See enhancements/topologies/mutable-topology.md and
// OCPEDGE-2952.
//
// The suite assumes a CI lane has already provisioned and joined the two
// additional control-plane nodes and that CEO has independently scaled etcd
// towards 3 voting members -- provisioning nodes from inside a Ginkgo test is
// not possible on platform:none. This suite waits for that state, then drives
// the transition itself.
var _ = g.Describe("[sig-etcd][sig-node][OCPFeatureGate:MutableTopology][Suite:openshift/topology-transition][Serial][Disruptive] Topology transition", g.Ordered, func() {
	oc := exutil.NewCLI("topology-transition").AsAdmin()

	g.BeforeEach(func(ctx context.Context) {
		infra, err := getInfrastructure(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "expected to retrieve Infrastructure/cluster")

		// Both topology fields are checked because they are exactly what the
		// controller's supported-transition matcher requires (see
		// buildSupportedTransitions's "From" descriptor in
		// cluster-config-operator/pkg/operator/topology_transition_controller/types.go);
		// a mismatch here would make the controller reject the request as
		// UnsupportedTransition rather than run preflight checks at all.
		if infra.Status.ControlPlaneTopology != configv1.SingleReplicaTopologyMode ||
			infra.Status.InfrastructureTopology != configv1.SingleReplicaTopologyMode {
			g.Skip("test requires a cluster starting in SingleReplica (SNO) topology")
		}
		if infra.Status.PlatformStatus == nil || infra.Status.PlatformStatus.Type != configv1.NonePlatformType {
			g.Skip("test requires platform:none")
		}
	})

	// This negative case is deliberately non-destructive and runs before the
	// happy-path transition below (enforced by the Ordered container). It
	// cannot rely on a precondition failing naturally -- by the time this
	// suite runs, the CI lane has already brought the cluster to the
	// preconditions-satisfied state described in the enhancement's test plan.
	// Instead it forces a precondition failure deterministically and
	// reversibly: cordoning control-plane node(s) drops the schedulable
	// control-plane count below the required 3, which
	// validateControlPlaneNodesSchedulable in the topology transition
	// controller rejects during preflight. See
	// cluster-config-operator/pkg/operator/topology_transition_controller.
	g.It("withholds admission when a control plane node is not schedulable [Timeout:30m][apigroup:config.openshift.io][apigroup:operator.openshift.io]", func(ctx context.Context) {
		// Uses the same dual-label (node-role.kubernetes.io/control-plane OR
		// the legacy node-role.kubernetes.io/master) control-plane detection
		// as checkControlPlaneNodePreconditions below, rather than
		// edgeutils.GetNodes(LabelNodeRoleControlPlane), which only selects
		// the new label and would undercount control-plane nodes on a
		// cluster still using the legacy label.
		nodes, err := listControlPlaneNodes(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(nodes)).To(o.BeNumerically(">=", 1), "expected at least one control plane node to cordon")

		// Only nodes that are already schedulable are candidates for
		// cordoning. listControlPlaneNodes can include nodes that are
		// already unschedulable for an unrelated reason; if one of those
		// were selected, the cordon patch would be a no-op that still
		// succeeds, so it would get recorded for cleanup and later
		// uncordoned -- mutating a node this test never actually changed.
		schedulableNodes := make([]corev1.Node, 0, len(nodes))
		for _, node := range nodes {
			if !node.Spec.Unschedulable {
				schedulableNodes = append(schedulableNodes, node)
			}
		}

		// Cordon enough of the schedulable nodes to leave at most 2
		// schedulable, guaranteeing validateControlPlaneNodesSchedulable(3)
		// fails regardless of how many control-plane nodes the lane has
		// joined by this point in the suite. Cordoning a fixed count of
		// exactly 1 would be insufficient if the lane over-provisioned
		// beyond 3 nodes: with 4+ schedulable nodes, cordoning only 1 would
		// still leave 3 schedulable, the check would pass, and (if other
		// preflights also passed) the controller could admit a real,
		// irreversible transition instead of rejecting this negative test's
		// request. If 2 or fewer nodes are already schedulable, the
		// precondition is already naturally failing, so nothing needs to be
		// cordoned at all.
		cordonCount := max(len(schedulableNodes)-2, 0)

		// cordonedNodes is declared, and both cleanups are registered, BEFORE
		// any cordon is attempted, and a node's name is appended to it only
		// once its own cordon succeeds. This ensures that if cordoning a
		// later node fails, every node cordoned so far is still uncordoned by
		// the registered cleanup -- otherwise a partial failure here would
		// leave earlier nodes permanently cordoned for the rest of this
		// [Serial] suite, since a later g.DeferCleanup call registered after
		// the failure would never run.
		cordonedNodes := make([]string, 0, cordonCount)

		// Registered as two independent DeferCleanup calls (run LIFO, like a
		// Go defer stack) rather than one function with two fail-fast
		// Expects, so that a failure in one step cannot prevent the other
		// from running. The order matters: uncordoning is registered FIRST
		// so it runs LAST, after the spec reset. If uncordon ran first, the
		// live controller (which reconciles independently of this test on
		// its own resync loop) could observe every precondition satisfied
		// while spec.controlPlaneTopology still requested HighlyAvailable,
		// and admit a real transition before the spec reset below ever runs
		// -- turning this "deliberately non-destructive" negative test into
		// an accidental trigger of the real one-way transition.
		g.DeferCleanup(func(ctx context.Context) {
			g.By("uncordoning the control plane node(s)")
			for _, name := range cordonedNodes {
				o.Expect(setNodeSchedulable(ctx, oc, name, true)).To(o.Succeed())
			}
		})
		g.DeferCleanup(func(ctx context.Context) {
			g.By("resetting spec.controlPlaneTopology back to SingleReplica")
			o.Expect(patchControlPlaneTopology(ctx, oc, configv1.SingleReplicaTopologyMode)).To(o.Succeed())
		})

		g.By("cordoning control plane node(s) to force a preflight failure")
		for i := range cordonCount {
			name := schedulableNodes[i].Name
			err := setNodeSchedulable(ctx, oc, name, false)
			if err == nil {
				cordonedNodes = append(cordonedNodes, name)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		// See nodeInformerPropagationWait's doc comment: give the controller's
		// node informer a chance to observe the cordon before requesting a
		// transition, so preflight evaluation cannot race ahead on stale
		// node state and admit a real transition instead of rejecting it.
		time.Sleep(nodeInformerPropagationWait)

		g.By("requesting a transition to HighlyAvailable")
		o.Expect(patchControlPlaneTopology(ctx, oc, configv1.HighlyAvailableTopologyMode)).To(o.Succeed())

		g.By("expecting the controller to withhold admission with PreflightCheckFailed")
		// Checking Status and Reason together in a single predicate (rather
		// than waiting on Status alone and inspecting Reason afterwards) is
		// required for correctness here: on a freshly idle cluster the
		// Progressing condition is already False (Reason=AsExpected) before
		// this test's patch takes effect, so a Status-only wait would return
		// immediately on that stale value instead of waiting for the
		// controller to actually run preflight and reject the request.
		progressing, _, err := waitForTransitionConditions(ctx, oc, admissionWaitTimeout, func(progressing, _ *operatorv1.OperatorCondition) bool {
			return progressing != nil && progressing.Status == operatorv1.ConditionFalse && progressing.Reason == preflightCheckFailedReason
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(progressing).NotTo(o.BeNil())
		o.Expect(progressing.Reason).To(o.Equal(preflightCheckFailedReason))

		g.By("confirming the topology status did not change")
		infra, err := getInfrastructure(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(infra.Status.ControlPlaneTopology).To(o.Equal(configv1.SingleReplicaTopologyMode))
	})

	g.It("transitions a SNO cluster to HA compact (3-node) [Timeout:150m][apigroup:config.openshift.io][apigroup:operator.openshift.io]", func(ctx context.Context) {
		g.By("waiting for the CI lane to have joined 3 ready, schedulable control plane nodes with no dedicated workers")
		o.Eventually(func() error {
			return checkControlPlaneNodePreconditions(ctx, oc)
		}).WithTimeout(preconditionWaitTimeout).WithPolling(15 * time.Second).Should(o.Succeed())

		g.By("waiting for etcd to reach 3 voting members")
		etcdClientFactory := etcdhelpers.NewEtcdClientFactory(oc.KubeClient())
		err := etcdhelpers.EnsureVotingMembersCount(ctx, g.GinkgoT(), etcdClientFactory, oc.KubeClient(), requiredEtcdVotingMembers)
		o.Expect(err).NotTo(o.HaveOccurred(), "expected etcd to reach 3 voting members before the transition can be requested")

		// Mirrors the controller's own validateClusterOperatorsStable
		// preflight check. Without this, operators left unstable by a prior
		// test (e.g. the negative test's cordon/uncordon immediately before
		// this one, since both run in the same Ordered container) would
		// surface as a confusing PreflightCheckFailed on the admission
		// assertion below instead of a clear failure here.
		g.By("waiting for cluster operators to be stable before requesting the transition")
		o.Expect(coutil.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), int(clusterOperatorStabilityTimeout.Minutes()))).To(o.Succeed())

		g.By("deploying a baseline workload to confirm availability survives the transition")
		o.Expect(createBaselineWorkload(ctx, oc)).To(o.Succeed())
		o.Expect(exutil.WaitForDeploymentReadyWithTimeout(oc, baselineWorkloadName, oc.Namespace(), -1, 5*time.Minute)).To(o.Succeed())

		g.By("requesting a transition to HighlyAvailable")
		o.Expect(patchControlPlaneTopology(ctx, oc, configv1.HighlyAvailableTopologyMode)).To(o.Succeed())

		// The controller writes Progressing and Upgradeable together in a
		// single status update on admission, so both are checked from one
		// fetch per poll (see getTransitionConditions) rather than with two
		// separately polled waits.
		g.By("expecting the controller to admit the transition (Progressing=True, Upgradeable=False)")
		admitProgressing, admitUpgradeable, err := waitForTransitionConditions(ctx, oc, admissionWaitTimeout, func(progressing, upgradeable *operatorv1.OperatorCondition) bool {
			return progressing != nil && progressing.Status == operatorv1.ConditionTrue &&
				upgradeable != nil && upgradeable.Status == operatorv1.ConditionFalse
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "last observed conditions: progressing=%+v upgradeable=%+v", admitProgressing, admitUpgradeable)

		g.By("confirming status.controlPlaneTopology/infrastructureTopology converge to HighlyAvailable")
		o.Expect(waitForControlPlaneTopology(ctx, oc, configv1.HighlyAvailableTopologyMode, statusConvergeTimeout)).To(o.Succeed())

		// Same reasoning as admission above: completion also flips both
		// conditions together (see checkClusterReconciliation in the
		// controller), so one consolidated wait covers both.
		g.By("waiting for the controller to report the transition complete (Progressing=False, Upgradeable=True)")
		completeProgressing, completeUpgradeable, err := waitForTransitionConditions(ctx, oc, completionWaitTimeout, func(progressing, upgradeable *operatorv1.OperatorCondition) bool {
			return progressing != nil && progressing.Status == operatorv1.ConditionFalse &&
				upgradeable != nil && upgradeable.Status == operatorv1.ConditionTrue
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "last observed conditions: progressing=%+v upgradeable=%+v", completeProgressing, completeUpgradeable)

		g.By("waiting for all cluster operators to settle post-transition")
		o.Expect(coutil.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), int(operatorSettleTimeout.Minutes()))).To(o.Succeed())

		g.By("confirming the baseline workload is still available")
		o.Expect(exutil.WaitForDeploymentReadyWithTimeout(oc, baselineWorkloadName, oc.Namespace(), -1, 2*time.Minute)).To(o.Succeed())
	})
})

// checkControlPlaneNodePreconditions returns nil once exactly
// requiredControlPlaneNodes control plane nodes are Ready and schedulable and
// no dedicated worker nodes are present, mirroring the preflight checks the
// topology transition controller itself enforces. Note the controller's own
// validateControlPlaneNodeCount accepts >=3, but this suite is specifically
// scoped to the SNO -> HA *compact 3-node* transition (not general topology
// tooling), so a lane that over-provisions beyond 3 control-plane nodes is
// a configuration mismatch worth failing on rather than silently accepting.
func checkControlPlaneNodePreconditions(ctx context.Context, oc *exutil.CLI) error {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	readySchedulableControlPlane := 0
	dedicatedWorkers := 0
	for _, node := range nodes.Items {
		isControlPlane := isControlPlaneNode(node.Labels)
		_, isWorker := node.Labels["node-role.kubernetes.io/worker"]

		if !isControlPlane && isWorker {
			dedicatedWorkers++
			continue
		}
		if isControlPlane && !node.Spec.Unschedulable && nodeIsReady(node) {
			readySchedulableControlPlane++
		}
	}

	if dedicatedWorkers != 0 {
		return fmt.Errorf("expected no dedicated worker nodes, found %d", dedicatedWorkers)
	}
	if readySchedulableControlPlane != requiredControlPlaneNodes {
		return fmt.Errorf("expected %d ready, schedulable control plane nodes, found %d", requiredControlPlaneNodes, readySchedulableControlPlane)
	}
	return nil
}

func nodeIsReady(node corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

// createBaselineWorkload creates a minimal Deployment used to confirm basic
// workload availability holds through the transition.
func createBaselineWorkload(ctx context.Context, oc *exutil.CLI) error {
	var replicas int32 = 2
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      baselineWorkloadName,
			Namespace: oc.Namespace(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": baselineWorkloadName},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": baselineWorkloadName},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    "workload",
						Image:   image.ShellImage(),
						Command: []string{"sleep", "infinity"},
					}},
				},
			},
		},
	}
	_, err := oc.KubeClient().AppsV1().Deployments(oc.Namespace()).Create(ctx, deployment, metav1.CreateOptions{})
	return err
}
