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

	selected, err := selectTransition(transitions, target)
	if err != nil {
		registerTransitionTargetError(err)
		return
	}
	registerTransitionTest(selected)
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

// registerTransitionTest registers one spec. Its setup must reject and restore
// a transition before the one-way transition in It can start.
func registerTransitionTest(spec TransitionSpec) {
	g.Describe("[sig-etcd][sig-node][OCPFeatureGate:MutableTopology][Suite:openshift/topology-transitions][Serial][Disruptive] Topology transition", func() {
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
		})

		// The rejection phase is deliberately non-destructive setup. It cannot
		// rely on a precondition failing naturally -- by the time this
		// suite runs, the CI lane has already brought the cluster to the
		// preconditions-satisfied state described in the enhancement's test plan.
		// Instead it forces a precondition failure deterministically and
		// reversibly: cordoning control-plane node(s) drops the schedulable
		// control-plane count below spec.RequiredControlPlaneNodes, which
		// validateControlPlaneNodesSchedulable in the topology transition
		// controller rejects during preflight. See
		// cluster-config-operator/pkg/operator/topology_transition_controller.
		// Both phases share a test process and the It's 220m budget (70m for
		// setup, 150m for the transition). A setup failure prevents the It from
		// running; separate specs would be shuffled and run in separate processes.
		g.BeforeEach(func(ctx context.Context) {
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

			// cordonedNodes is declared, and fallback cleanup is registered, BEFORE
			// any cordon is attempted, and a node's name is appended to it only
			// once its own cordon succeeds. This ensures that if cordoning a
			// later node fails, every node cordoned so far is still uncordoned by
			// the registered cleanup after a partial failure.
			cordonedNodes := make([]string, 0, cordonCount)

			requestAttempted := false
			restored := false
			restore := func(ctx context.Context) error {
				return restoreRejectedTransition(ctx, requestAttempted,
					func(ctx context.Context) error { return patchControlPlaneTopology(ctx, oc, originalTopology) },
					func(ctx context.Context) error {
						progressing, _, err := waitForTransitionConditions(ctx, oc, idleWaitTimeout, func(progressing, _ *operatorv1.OperatorCondition) bool {
							return progressing != nil && progressing.Reason == reasonAsExpected
						})
						if err != nil {
							return fmt.Errorf("last progressing condition %s: %w", conditionSummary(progressing), err)
						}
						return nil
					},
					cordonedNodes,
					func(ctx context.Context, name string) error { return setNodeSchedulable(ctx, oc, name, true) },
				)
			}
			// If the negative phase fails, retry restoration. Never reset the
			// request after the real transition has started.
			g.DeferCleanup(func(ctx context.Context) {
				if !restored {
					o.Expect(restore(ctx)).To(o.Succeed(), "failed to restore the rejected transition")
				}
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
			requestAttempted = true
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

			g.By("restoring the topology request and waiting for idle before uncordoning")
			o.Expect(restore(ctx)).To(o.Succeed(), "failed to restore the rejected transition; will retry during cleanup")
			restored = true
		})

		g.It("completes "+spec.Name+" [Timeout:220m][apigroup:config.openshift.io][apigroup:operator.openshift.io]", func(ctx context.Context) {
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
// upgrade. The setup and It each call this before their phase. For the
// happy path it lets the request get admitted; for the negative phase it
// guarantees cordoning a node is the ONLY unmet precondition. Without it,
// the test could pass for the wrong reason.
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
	// check. Otherwise, operators left unstable by the rejection phase's
	// cordon/uncordon would cause a confusing admission failure.
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
