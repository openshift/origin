package topology_transitions

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

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
	// The CI lane is expected to have already brought the cluster to the
	// preconditions-satisfied state before this suite runs, so this is a
	// bounded sanity wait, not the lane's own (much longer) provisioning
	// budget.
	preconditionWaitTimeout = 20 * time.Minute

	// idleWaitTimeout bounds how long the negative test's cleanup waits for
	// the controller to observe a withdrawn transition request (Progressing
	// Reason=AsExpected) before uncordoning nodes. See the cleanup's own
	// comment for why uncordoning must not race ahead of this.
	idleWaitTimeout = 5 * time.Minute

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

	baselineWorkloadName = "topology-transitions-baseline"
)

// init registers the transition specs selected by the CI lane's target values.
func init() {
	target, err := transitionTargetFromEnvironment(os.Getenv)
	if err != nil {
		registerTransitionTargetError(err)
		return
	}

	selected := matchingTransitions(transitions, target)
	if len(selected) == 0 {
		registerTransitionTargetError(fmt.Errorf(
			"no transition matches control-plane topology %q, infrastructure topology %q, and TARGET_HA_COMPACT=%t",
			target.ControlPlaneTopology, target.InfrastructureTopology, target.HACompact,
		))
		return
	}
	for _, t := range selected {
		registerTransitionTests(t)
	}
}

// transitionTargetFromEnvironment reads and parses the CI lane's target values.
func transitionTargetFromEnvironment(getenv func(string) string) (transitionTarget, error) {
	return parseTransitionTarget(
		getenv(targetControlPlaneTopologyEnvVar),
		getenv(targetInfrastructureTopologyEnvVar),
		getenv(targetHACompactEnvVar),
	)
}

// registerTransitionTargetError makes invalid lane configuration fail in this suite.
func registerTransitionTargetError(err error) {
	g.Describe("[OCPFeatureGate:MutableTopology][Suite:openshift/topology-transitions] transition target configuration", func() {
		g.It("selects a configured transition", func() {
			o.Expect(err).NotTo(o.HaveOccurred(), "transition target configuration is invalid: %v", err)
		})
	})
}

// registerTransitionTests registers the negative and happy-path specs for one
// TransitionSpec row. Each row gets its own top-level Ordered container so
// that one row's happy-path failure (which leaves the cluster mutated -- these
// are one-way transitions) cannot skip a sibling row's unrelated specs.
func registerTransitionTests(spec TransitionSpec) {
	g.Describe("[sig-etcd][sig-node][OCPFeatureGate:MutableTopology][Suite:openshift/topology-transitions][Serial][Disruptive] Topology transition", g.Ordered, func() {
		oc := exutil.NewCLI("topology-transitions").AsAdmin()

		g.BeforeEach(func(ctx context.Context) {
			infra, err := getInfrastructure(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "expected to retrieve Infrastructure/cluster")

			if !spec.matchesFrom(infra) {
				fromPlatformType := "any"
				if spec.From.PlatformStatus != nil {
					fromPlatformType = platformType(spec.From.PlatformStatus)
				}
				g.Skip(fmt.Sprintf(
					"transition %q requires starting state controlPlaneTopology=%s infrastructureTopology=%s platformType=%s; cluster is currently controlPlaneTopology=%s infrastructureTopology=%s platformType=%s",
					spec.Name, spec.From.ControlPlaneTopology, spec.From.InfrastructureTopology, fromPlatformType,
					infra.Status.ControlPlaneTopology, infra.Status.InfrastructureTopology, platformType(infra.Status.PlatformStatus)))
			}

			// See detectChain's doc comment: this must run only for a row whose
			// starting state actually matched (i.e. after the skip above), and
			// converts a silently chained second transition into an immediate,
			// loud spec failure instead of letting Ginkgo execute it.
			o.Expect(detectChain(spec.Name)).To(o.Succeed(), "refusing to chain transition %q in one suite invocation", spec.Name)
		})

		// This negative case is deliberately non-destructive and runs before the
		// happy-path transition below (enforced by the Ordered container). It
		// cannot rely on a precondition failing naturally -- by the time this
		// suite runs, the CI lane has already brought the cluster to the
		// preconditions-satisfied state described in the enhancement's test plan.
		// Instead it forces a precondition failure deterministically and
		// reversibly: cordoning control-plane node(s) drops the schedulable
		// control-plane count below spec.RequiredControlPlaneNodes, which
		// validateControlPlaneNodesSchedulable in the topology transition
		// controller rejects during preflight. See
		// cluster-config-operator/pkg/operator/topology_transition_controller.
		g.It("withholds admission when a control plane node is not schedulable [Timeout:30m][apigroup:config.openshift.io][apigroup:operator.openshift.io]", func(ctx context.Context) {
			// Establishing the full set of preconditions first guarantees that
			// cordoning below is the ONLY unmet preflight check afterward.
			// Without this, if the lane hadn't yet reached its steady state,
			// cordonCount could land on zero and this test would pass for the
			// wrong reason (some other, unrelated precondition already failing).
			waitForTransitionPreconditions(ctx, oc, spec)

			// Captured so cleanup can restore the actual starting value rather
			// than assuming a specific mode -- a freshly-installed cluster can
			// have an empty spec.controlPlaneTopology.
			infra, err := getInfrastructure(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "expected to retrieve Infrastructure/cluster before cordoning nodes")
			originalTopology := infra.Spec.ControlPlaneTopology

			// Uses the same dual-label (node-role.kubernetes.io/control-plane OR
			// the legacy node-role.kubernetes.io/master) control-plane detection
			// as checkControlPlaneNodePreconditions, rather than
			// edgeutils.GetNodes(LabelNodeRoleControlPlane), which only selects
			// the new label and would undercount control-plane nodes on a
			// cluster still using the legacy label.
			nodes, err := listControlPlaneNodes(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "expected to list control-plane nodes to cordon")
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

			// Cordon enough of the schedulable nodes to leave at most
			// spec.RequiredControlPlaneNodes-1 schedulable, guaranteeing the
			// controller's schedulability preflight check fails regardless of
			// how many control-plane nodes the lane has joined by this point in
			// the suite. Cordoning a fixed count of exactly 1 would be
			// insufficient if the lane over-provisioned beyond
			// spec.RequiredControlPlaneNodes: with more schedulable nodes than
			// required, cordoning only 1 would still leave enough schedulable,
			// the check would pass, and (if other preflights also passed) the
			// controller could admit a real, irreversible transition instead of
			// rejecting this negative test's request. If
			// spec.RequiredControlPlaneNodes-1 or fewer nodes are already
			// schedulable, the precondition is already naturally failing, so
			// nothing needs to be cordoned at all.
			cordonCount := max(len(schedulableNodes)-(spec.RequiredControlPlaneNodes-1), 0)

			// cordonedNodes is declared, and both cleanups are registered, BEFORE
			// any cordon is attempted, and a node's name is appended to it only
			// once its own cordon succeeds. This ensures that if cordoning a
			// later node fails, every node cordoned so far is still uncordoned by
			// the registered cleanup -- otherwise a partial failure here would
			// leave earlier nodes permanently cordoned for the rest of this
			// suite, since a later g.DeferCleanup call registered after the
			// failure would never run.
			cordonedNodes := make([]string, 0, cordonCount)

			// Registered as two independent DeferCleanup calls (run LIFO, like a
			// Go defer stack) rather than one function with two fail-fast
			// Expects, so that a failure in one step cannot prevent the other
			// from running. The order matters: uncordoning is registered FIRST
			// so it runs LAST, after the spec reset. If uncordon ran first, the
			// live controller (which reconciles independently of this test on
			// its own resync loop) could observe every precondition satisfied
			// while spec.controlPlaneTopology still requested the target mode,
			// and admit a real transition before the spec reset below ever runs
			// -- turning this "deliberately non-destructive" negative test into
			// an accidental trigger of the real one-way transition.
			g.DeferCleanup(func(ctx context.Context) {
				// Uncordoning before the controller has observed the spec reset
				// below could let it see every precondition satisfied while
				// still processing a stale transition request, admitting a real
				// transition. Waiting for Reason=AsExpected confirms the
				// controller has withdrawn the rejected request and gone idle.
				// If it never does, fail here and leave the node(s) cordoned
				// rather than risk uncordoning into a still-pending transition.
				g.By("waiting for the controller to observe the spec reset and report idle before uncordoning")
				progressing, _, err := waitForTransitionConditions(ctx, oc, idleWaitTimeout, func(progressing, _ *operatorv1.OperatorCondition) bool {
					return progressing != nil && progressing.Reason == reasonAsExpected
				})
				o.Expect(err).NotTo(o.HaveOccurred(), "controller did not return to idle after spec reset; leaving nodes cordoned: last progressing condition %s", conditionSummary(progressing))

				g.By("uncordoning the control plane node(s)")
				for _, name := range cordonedNodes {
					err := setNodeSchedulable(ctx, oc, name, true)
					o.Expect(err == nil).To(o.BeTrue(), "failed to uncordon a control-plane node")
				}
			})
			g.DeferCleanup(func(ctx context.Context) {
				g.By("resetting spec.controlPlaneTopology back to its original value")
				o.Expect(patchControlPlaneTopology(ctx, oc, originalTopology)).To(o.Succeed(), "failed to restore original spec.controlPlaneTopology")
			})

			g.By("cordoning control plane node(s) to force a preflight failure")
			for i := range cordonCount {
				name := schedulableNodes[i].Name
				err := setNodeSchedulable(ctx, oc, name, false)
				if err == nil {
					cordonedNodes = append(cordonedNodes, name)
				}
				o.Expect(err == nil).To(o.BeTrue(), "failed to cordon a control-plane node")
			}

			// See nodeInformerPropagationWait's doc comment: give the controller's
			// node informer a chance to observe the cordon before requesting a
			// transition, so preflight evaluation cannot race ahead on stale
			// node state and admit a real transition instead of rejecting it.
			time.Sleep(nodeInformerPropagationWait)

			g.By("requesting a transition to " + string(spec.To.ControlPlaneTopology))
			o.Expect(patchControlPlaneTopology(ctx, oc, spec.To.ControlPlaneTopology)).To(o.Succeed(), "failed to request transition to %s", spec.To.ControlPlaneTopology)

			g.By("expecting the controller to withhold admission with PreflightCheckFailed")
			// Checking Status and Reason together in a single predicate (rather
			// than waiting on Status alone and inspecting Reason afterwards) is
			// required for correctness here: on a freshly idle cluster the
			// Progressing condition is already False (Reason=AsExpected) before
			// this test's patch takes effect, so a Status-only wait would return
			// immediately on that stale value instead of waiting for the
			// controller to actually run preflight and reject the request. The
			// Message is also checked so this test only passes if the
			// controller rejected for the specific reason it is exercising, not
			// some other, coincidentally-also-failing preflight check.
			progressing, _, err := waitForTransitionConditions(ctx, oc, spec.AdmissionWaitTimeout, func(progressing, _ *operatorv1.OperatorCondition) bool {
				return progressing != nil && progressing.Status == operatorv1.ConditionFalse &&
					progressing.Reason == preflightCheckFailedReason &&
					strings.Contains(progressing.Message, spec.ExpectedScheduleFailureSubstring)
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "expected preflight rejection for an unschedulable control plane")
			o.Expect(progressing).NotTo(o.BeNil(), "expected a Progressing condition after the rejection")
			o.Expect(progressing.Reason).To(o.Equal(preflightCheckFailedReason), "expected the controller to reject the transition during preflight")
			o.Expect(strings.Contains(progressing.Message, spec.ExpectedScheduleFailureSubstring)).To(o.BeTrue(),
				"expected the preflight rejection to identify %q (condition %s)", spec.ExpectedScheduleFailureSubstring, conditionSummary(progressing))

			g.By("confirming the topology status did not change")
			infra, err = getInfrastructure(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "expected to retrieve Infrastructure/cluster after preflight rejection")
			o.Expect(infra.Status.ControlPlaneTopology).To(o.Equal(spec.From.ControlPlaneTopology), "controlPlaneTopology changed after rejected transition")
			o.Expect(infra.Status.InfrastructureTopology).To(o.Equal(spec.From.InfrastructureTopology), "infrastructureTopology changed after rejected transition")
		})

		g.It("transitions the cluster to the target topology [Timeout:150m][apigroup:config.openshift.io][apigroup:operator.openshift.io]", func(ctx context.Context) {
			waitForTransitionPreconditions(ctx, oc, spec)

			g.By("deploying a baseline workload to confirm availability survives the transition")
			o.Expect(createBaselineWorkload(ctx, oc)).To(o.Succeed(), "failed to create baseline workload in namespace %q", oc.Namespace())
			o.Expect(exutil.WaitForDeploymentReadyWithTimeout(oc, baselineWorkloadName, oc.Namespace(), -1, 5*time.Minute)).To(o.Succeed(), "baseline workload %q did not become ready before the transition", baselineWorkloadName)

			g.By("requesting a transition to " + string(spec.To.ControlPlaneTopology))
			o.Expect(patchControlPlaneTopology(ctx, oc, spec.To.ControlPlaneTopology)).To(o.Succeed(), "failed to request transition to %s", spec.To.ControlPlaneTopology)

			// The controller writes Progressing and Upgradeable together in a
			// single status update on admission, so both are checked from one
			// fetch per poll (see getTransitionConditions) rather than with two
			// separately polled waits.
			g.By("expecting the controller to admit the transition (Progressing=True, Upgradeable=False)")
			admitProgressing, admitUpgradeable, err := waitForTransitionConditions(ctx, oc, spec.AdmissionWaitTimeout, func(progressing, upgradeable *operatorv1.OperatorCondition) bool {
				return progressing != nil && progressing.Status == operatorv1.ConditionTrue &&
					upgradeable != nil && upgradeable.Status == operatorv1.ConditionFalse
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "controller did not admit transition; last conditions: progressing=%s upgradeable=%s", conditionSummary(admitProgressing), conditionSummary(admitUpgradeable))

			g.By("confirming status.controlPlaneTopology/infrastructureTopology converge to the target topology")
			o.Expect(waitForTopology(ctx, oc, spec.To.ControlPlaneTopology, spec.ToInfrastructureTopology, spec.StatusConvergeTimeout)).To(o.Succeed(), "topology status did not reach controlPlaneTopology=%s and infrastructureTopology=%s", spec.To.ControlPlaneTopology, spec.ToInfrastructureTopology)

			// Same reasoning as admission above: completion also flips both
			// conditions together (see checkClusterReconciliation in the
			// controller), so one consolidated wait covers both.
			g.By("waiting for the controller to report the transition complete (Progressing=False, Upgradeable=True)")
			completeProgressing, completeUpgradeable, err := waitForTransitionConditions(ctx, oc, spec.CompletionWaitTimeout, func(progressing, upgradeable *operatorv1.OperatorCondition) bool {
				return progressing != nil && progressing.Status == operatorv1.ConditionFalse &&
					upgradeable != nil && upgradeable.Status == operatorv1.ConditionTrue
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "controller did not complete transition; last conditions: progressing=%s upgradeable=%s", conditionSummary(completeProgressing), conditionSummary(completeUpgradeable))

			g.By("waiting for all cluster operators to settle post-transition")
			o.Expect(coutil.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), int(spec.OperatorSettleTimeout.Minutes()))).To(o.Succeed(), "cluster operators did not settle after the transition")

			g.By("confirming the baseline workload is still available")
			o.Expect(exutil.WaitForDeploymentReadyWithTimeout(oc, baselineWorkloadName, oc.Namespace(), -1, 2*time.Minute)).To(o.Succeed(), "baseline workload %q was not ready after the transition", baselineWorkloadName)
		})
	})
}

// waitForTransitionPreconditions blocks until the cluster satisfies every
// precondition the topology transition controller's own preflight checks
// require for spec: node topology (count/ready/schedulable/dual-role), etcd
// health, cluster operator stability, and no in-progress cluster version
// upgrade. Both specs in registerTransitionTests call this first. For the
// happy path it's what lets the transition request below actually get
// admitted; for the negative test it guarantees that cordoning a node is the
// ONLY unmet precondition afterward -- otherwise, if the lane hadn't yet
// reached steady state, cordonCount could land on zero and that test would
// pass for the wrong reason.
func waitForTransitionPreconditions(ctx context.Context, oc *exutil.CLI, spec TransitionSpec) {
	g.By("waiting for the CI lane to reach the required control plane node shape")
	o.Eventually(func() error {
		return checkControlPlaneNodePreconditions(ctx, oc, spec)
	}).WithTimeout(preconditionWaitTimeout).WithPolling(15*time.Second).Should(o.Succeed(), "required control-plane node shape did not become ready")

	g.By(fmt.Sprintf("waiting for etcd to reach %d voting members", spec.RequiredEtcdVotingMembers))
	etcdClientFactory := etcdhelpers.NewEtcdClientFactory(oc.KubeClient())
	err := etcdhelpers.EnsureVotingMembersCount(ctx, g.GinkgoT(), etcdClientFactory, oc.KubeClient(), spec.RequiredEtcdVotingMembers)
	o.Expect(err).NotTo(o.HaveOccurred(), "expected etcd to reach %d voting members before the transition can be requested", spec.RequiredEtcdVotingMembers)

	// EnsureVotingMembersCount above only counts members; it explicitly
	// doesn't evaluate health, so this covers the quorum/health dimension
	// the controller's own validateEtcdQuorum/validateEtcdNotProgressing
	// preflight checks require.
	g.By("waiting for etcd members to be available and not progressing")
	o.Eventually(func() error {
		return checkEtcdHealthy(ctx, oc)
	}).WithTimeout(preconditionWaitTimeout).WithPolling(15*time.Second).Should(o.Succeed(), "etcd members did not become available and stop progressing")

	// Mirrors the controller's own validateClusterOperatorsStable preflight
	// check. Without this, operators left unstable by a prior test (e.g. the
	// negative test's cordon/uncordon, since both specs run in the same
	// Ordered container) would surface as a confusing PreflightCheckFailed on
	// the transition-admission assertion below instead of a clear failure
	// here.
	g.By("waiting for cluster operators to be stable")
	o.Expect(coutil.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), int(spec.ClusterOperatorStabilityTimeout.Minutes()))).To(o.Succeed(), "cluster operators did not settle before the transition")

	// Mirrors the controller's own validateNoClusterVersionUpgradeInProgress
	// preflight check.
	g.By("confirming no cluster version upgrade is in progress")
	o.Expect(checkNoUpgradeInProgress(ctx, oc)).To(o.Succeed(), "a cluster-version upgrade is in progress")
}

// createBaselineWorkload creates a minimal Deployment used as a before/after
// smoke check that basic workload scheduling still works once the
// transition completes -- the enhancement makes no availability guarantee
// *during* a transition, so this is intentionally not a continuous
// availability monitor.
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
					// Soft (ScheduleAnyway), not required: this workload is
					// created before the transition, so a hard constraint
					// could leave a replica unschedulable if the starting
					// topology has too few eligible nodes. Once the target
					// topology is ready, this best-effort constraint encourages
					// the replicas to spread across available nodes
					// rather than staying co-located on the original node.
					TopologySpreadConstraints: []corev1.TopologySpreadConstraint{{
						MaxSkew:           1,
						TopologyKey:       "kubernetes.io/hostname",
						WhenUnsatisfiable: corev1.ScheduleAnyway,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": baselineWorkloadName},
						},
					}},
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
