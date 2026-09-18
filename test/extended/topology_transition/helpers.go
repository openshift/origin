// Package topology_transition contains e2e tests that trigger and validate
// control-plane topology transitions orchestrated by the topology transition
// controller in cluster-config-operator, gated behind the MutableTopology
// feature gate. See enhancements/topologies/mutable-topology.md.
package topology_transition

import (
	"context"
	"fmt"
	"sync"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	infraName = "cluster"

	// operatorConfigName is the name of the cluster-scoped
	// configs.operator.openshift.io object that the topology transition
	// controller writes conditions to. It happens to also be "cluster",
	// same as infraName, but the two are independent API objects/contracts,
	// so they get their own named constant rather than sharing infraName.
	operatorConfigName = "cluster"

	// Condition types written by the topology transition controller onto
	// configs.operator.openshift.io/cluster. These -- and their Reason strings
	// -- are diagnostic signals only, not a stable API contract: OCPEDGE-2958
	// explicitly redesigns transition-progress reporting as well-defined
	// InfrastructureStatus fields rather than dynamic condition types, because
	// consuming raw conditions was flagged as an anti-pattern in review of the
	// 5.0 carryover demo (cluster-config-operator#495). Prefer asserting
	// against status.controlPlaneTopology/infrastructureTopology, and switch
	// the primary in-progress signal to the new InfrastructureStatus fields
	// once they merge.
	transitionProgressingCondition = "TopologyTransitionControllerProgressing"
	transitionUpgradeableCondition = "TopologyTransitionControllerUpgradeable"

	// preflightCheckFailedReason is the Reason set on the transition
	// controller's conditions when it withholds admission because a
	// precondition (cluster operator stability, node counts, etcd quorum,
	// etc.) is not met.
	preflightCheckFailedReason = "PreflightCheckFailed"

	// reasonAsExpected is the Reason the controller reports on the
	// Progressing condition once it has observed a rejected/withdrawn
	// transition request and returned to idle.
	reasonAsExpected = "AsExpected"

	// etcdName is the name of the cluster-scoped etcds.operator.openshift.io
	// object the topology transition controller's etcd preflight checks
	// read. It happens to also be "cluster", like infraName and
	// operatorConfigName, but is an independent API object, so it gets its
	// own named constant.
	etcdName = "cluster"

	// Condition types on etcds.operator.openshift.io/cluster that the
	// controller's validateEtcdQuorum/validateEtcdNotProgressing preflight
	// checks read.
	etcdMembersAvailableCondition   = "EtcdMembersAvailable"
	etcdMembersProgressingCondition = "EtcdMembersProgressing"

	// clusterVersionName is the name of the cluster-scoped ClusterVersion
	// object the controller's validateNoClusterVersionUpgradeInProgress
	// preflight check reads.
	clusterVersionName = "version"
)

// getInfrastructure fetches the cluster-scoped Infrastructure object.
func getInfrastructure(ctx context.Context, oc *exutil.CLI) (*configv1.Infrastructure, error) {
	return oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, infraName, metav1.GetOptions{})
}

// getTransitionConditions fetches configs.operator.openshift.io/cluster once
// and returns both conditions written by the topology transition controller.
// Either return value is nil if that condition has not been reported yet.
// The controller always updates both conditions together in a single status
// write, so reading them from one fetch avoids racing between two separately
// polled reads of conditions that are meant to be evaluated as a pair.
func getTransitionConditions(ctx context.Context, oc *exutil.CLI) (progressing, upgradeable *operatorv1.OperatorCondition, err error) {
	config, err := oc.AdminOperatorClient().OperatorV1().Configs().Get(ctx, operatorConfigName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	for i := range config.Status.Conditions {
		switch config.Status.Conditions[i].Type {
		case transitionProgressingCondition:
			progressing = &config.Status.Conditions[i]
		case transitionUpgradeableCondition:
			upgradeable = &config.Status.Conditions[i]
		}
	}
	return progressing, upgradeable, nil
}

// checkEtcdHealthy mirrors the controller's own validateEtcdQuorum and
// validateEtcdNotProgressing preflight checks: it requires
// etcds.operator.openshift.io/cluster to report EtcdMembersAvailable=True
// and EtcdMembersProgressing=False. EnsureVotingMembersCount (used
// alongside this) only counts voting members and explicitly does not
// evaluate health, so this covers the health/quorum dimension the count
// check leaves out.
func checkEtcdHealthy(ctx context.Context, oc *exutil.CLI) error {
	etcd, err := oc.AdminOperatorClient().OperatorV1().Etcds().Get(ctx, etcdName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if available := v1helpers.FindOperatorCondition(etcd.Status.Conditions, etcdMembersAvailableCondition); available == nil || available.Status != operatorv1.ConditionTrue {
		return fmt.Errorf("etcd %s condition is not True: %+v", etcdMembersAvailableCondition, available)
	}
	if progressing := v1helpers.FindOperatorCondition(etcd.Status.Conditions, etcdMembersProgressingCondition); progressing == nil || progressing.Status != operatorv1.ConditionFalse {
		return fmt.Errorf("etcd %s condition is not False: %+v", etcdMembersProgressingCondition, progressing)
	}
	return nil
}

// checkNoUpgradeInProgress mirrors the controller's own
// validateNoClusterVersionUpgradeInProgress preflight check.
func checkNoUpgradeInProgress(ctx context.Context, oc *exutil.CLI) error {
	cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(ctx, clusterVersionName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	for _, cond := range cv.Status.Conditions {
		if cond.Type == configv1.OperatorProgressing && cond.Status == configv1.ConditionTrue {
			return fmt.Errorf("a cluster version upgrade is in progress: %s", cond.Message)
		}
	}
	return nil
}

// checkControlPlaneNodePreconditions returns nil once exactly
// spec.requiredControlPlaneNodes control plane nodes are Ready and
// schedulable (and, if spec.requireDualRoleControlPlane, also carry the
// worker role), and no dedicated worker nodes are present -- mirroring the
// preflight checks the topology transition controller itself enforces
// (validateControlPlaneNodeCount, validateControlPlaneNodesSchedulable,
// validateControlPlaneNodesReady, validateControlPlaneNodesAreWorkers,
// validateExactInfrastructureNodeCount). Note the controller's own
// validateControlPlaneNodeCount accepts >=N, but this suite intentionally
// requires an exact count, so a lane that over-provisions beyond
// spec.requiredControlPlaneNodes is a configuration mismatch worth failing
// on rather than silently accepting.
func checkControlPlaneNodePreconditions(ctx context.Context, oc *exutil.CLI, spec transitionSpec) error {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	readySchedulableControlPlane := 0
	dualRoleControlPlane := 0
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
			if isWorker {
				dualRoleControlPlane++
			}
		}
	}

	if dedicatedWorkers != 0 {
		return fmt.Errorf("expected no dedicated worker nodes, found %d", dedicatedWorkers)
	}
	if readySchedulableControlPlane != spec.requiredControlPlaneNodes {
		return fmt.Errorf("expected %d ready, schedulable control plane nodes, found %d", spec.requiredControlPlaneNodes, readySchedulableControlPlane)
	}
	if spec.requireDualRoleControlPlane && dualRoleControlPlane != spec.requiredControlPlaneNodes {
		return fmt.Errorf("expected %d ready, schedulable control plane nodes to also carry the worker role, found %d", spec.requiredControlPlaneNodes, dualRoleControlPlane)
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

// patchControlPlaneTopology patches spec.controlPlaneTopology on the cluster
// Infrastructure object -- this is how an administrator requests a topology
// transition today. There is no `oc adm` command for this yet (OCPEDGE-2960);
// the CLI is expected to wrap this same patch.
func patchControlPlaneTopology(ctx context.Context, oc *exutil.CLI, mode configv1.TopologyMode) error {
	data := fmt.Sprintf(`{"spec":{"controlPlaneTopology":%q}}`, mode)
	_, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Patch(ctx, infraName, types.MergePatchType, []byte(data), metav1.PatchOptions{})
	return err
}

// setNodeSchedulable cordons (schedulable=false) or uncordons
// (schedulable=true) a node by patching spec.unschedulable.
func setNodeSchedulable(ctx context.Context, oc *exutil.CLI, nodeName string, schedulable bool) error {
	data := fmt.Sprintf(`{"spec":{"unschedulable":%t}}`, !schedulable)
	_, err := oc.AdminKubeClient().CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, []byte(data), metav1.PatchOptions{})
	return err
}

// waitForControlPlaneTopology polls Infrastructure.Status until both
// controlPlaneTopology and infrastructureTopology equal want, or the timeout
// elapses. This is the durable, stable signal for transition completion.
func waitForControlPlaneTopology(ctx context.Context, oc *exutil.CLI, want configv1.TopologyMode, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 15*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		infra, err := getInfrastructure(ctx, oc)
		if err != nil {
			e2e.Logf("failed to get infrastructure, will retry: %v", err)
			return false, nil
		}
		if infra.Status.ControlPlaneTopology != want || infra.Status.InfrastructureTopology != want {
			e2e.Logf("waiting for topology to reach %s: controlPlaneTopology=%s infrastructureTopology=%s",
				want, infra.Status.ControlPlaneTopology, infra.Status.InfrastructureTopology)
			return false, nil
		}
		return true, nil
	})
}

// waitForTransitionConditions polls until check returns true for the current
// Progressing/Upgradeable condition pair (fetched together from a single
// object, per the race note on getTransitionConditions above), or the
// timeout elapses. It returns the last-observed pair regardless of outcome,
// so callers can assert on it for a more informative failure message.
func waitForTransitionConditions(ctx context.Context, oc *exutil.CLI, timeout time.Duration, check func(progressing, upgradeable *operatorv1.OperatorCondition) bool) (progressing, upgradeable *operatorv1.OperatorCondition, err error) {
	err = wait.PollUntilContextTimeout(ctx, 15*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		p, u, err := getTransitionConditions(ctx, oc)
		if err != nil {
			e2e.Logf("failed to get transition conditions, will retry: %v", err)
			return false, nil
		}
		progressing, upgradeable = p, u
		if !check(p, u) {
			e2e.Logf("transition conditions not yet as expected: progressing=%+v upgradeable=%+v", p, u)
			return false, nil
		}
		return true, nil
	})
	return progressing, upgradeable, err
}

// isControlPlaneNode returns true if the node carries either of the two
// labels used across the codebase to identify control-plane nodes.
func isControlPlaneNode(labels map[string]string) bool {
	_, master := labels["node-role.kubernetes.io/master"]
	_, controlPlane := labels["node-role.kubernetes.io/control-plane"]
	return master || controlPlane
}

// listControlPlaneNodes lists all nodes and returns those identified as
// control-plane by isControlPlaneNode. Callers needing only the
// node-role.kubernetes.io/control-plane label (e.g.
// edgeutils.GetNodes(LabelNodeRoleControlPlane)) would undercount on a
// cluster still using the legacy node-role.kubernetes.io/master label.
func listControlPlaneNodes(ctx context.Context, oc *exutil.CLI) ([]corev1.Node, error) {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	controlPlaneNodes := make([]corev1.Node, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		if isControlPlaneNode(node.Labels) {
			controlPlaneNodes = append(controlPlaneNodes, node)
		}
	}
	return controlPlaneNodes, nil
}

// exercisedTransitionName records which transitionSpec (by name) has already
// run in this suite invocation. Guarded by exercisedTransitionMu even though
// the suite runs with Parallelism: 1 (specs execute serially in one process),
// as a near-zero-cost defense against that assumption changing later.
var (
	exercisedTransitionMu   sync.Mutex
	exercisedTransitionName string
)

// detectChain returns a non-nil error if name differs from the transition
// already recorded as exercised in this suite invocation; otherwise it
// records name (idempotently) as the exercised transition and returns nil.
// Kept free of Ginkgo calls so it's unit-testable with plain go test;
// callers (see registerTransitionTests) translate a non-nil return into a
// Ginkgo spec failure via o.Expect(...).To(o.Succeed()), the same pattern
// used everywhere else in this package.
//
// This exists because transitions are one-way and irreversible, and a row's
// own happy path mutates the exact Infrastructure.Status field a later row's
// matchesFrom reads. Without this check, on a lane starting SNO with e.g.
// both an SNO->TNA/TNF row and a TNA/TNF->HA row registered, the SNO->TNA/TNF
// row's happy path could mutate status into a shape the TNA/TNF->HA row's
// matchesFrom also accepts, silently chaining a second, unintended,
// irreversible transition into the same suite invocation.
func detectChain(name string) error {
	exercisedTransitionMu.Lock()
	defer exercisedTransitionMu.Unlock()
	if exercisedTransitionName == "" {
		exercisedTransitionName = name
		return nil
	}
	if exercisedTransitionName != name {
		return fmt.Errorf(
			"transition %q's starting state now matches, but this suite invocation already "+
				"exercised transition %q -- refusing to silently chain a second one-way "+
				"transition in the same run (this indicates either a registration-order bug "+
				"or a misconfigured multi-leg CI lane)",
			name, exercisedTransitionName)
	}
	return nil
}
