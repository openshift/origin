// Package topology_transition contains e2e tests that trigger and validate
// control-plane topology transitions orchestrated by the topology transition
// controller in cluster-config-operator, gated behind the MutableTopology
// feature gate. See enhancements/topologies/mutable-topology.md.
package topology_transition

import (
	"context"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	infraName = "cluster"

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
	config, err := oc.AdminOperatorClient().OperatorV1().Configs().Get(ctx, infraName, metav1.GetOptions{})
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
