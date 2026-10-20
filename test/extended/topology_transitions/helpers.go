package topology_transitions

import (
	"context"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	infraName                       = "cluster"
	etcdName                        = "cluster"
	etcdMembersAvailableCondition   = "EtcdMembersAvailable"
	etcdMembersProgressingCondition = "EtcdMembersProgressing"
)

func getInfrastructure(ctx context.Context, oc *exutil.CLI) (*configv1.Infrastructure, error) {
	return oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, infraName, metav1.GetOptions{})
}

// Check the report and every check against the current Infrastructure generation.
func checkTransitionEvaluation(infra *configv1.Infrastructure, definition *transitionDefinition, preflight bool) error {
	evaluated := meta.FindStatusCondition(infra.Status.TopologyTransitionStatus.Conditions, configv1.TopologyTransitionsEvaluatedConditionType)
	if evaluated == nil || evaluated.Status != metav1.ConditionTrue || evaluated.ObservedGeneration != infra.Generation {
		return fmt.Errorf("waiting for a current TopologyTransitionsEvaluated=True result")
	}

	var matching *configv1.TopologyTransition
	for i := range infra.Status.TopologyTransitionStatus.Transitions {
		transition := &infra.Status.TopologyTransitionStatus.Transitions[i]
		if transition.Source == definition.sourceTopology() && transition.Target == definition.Target {
			if matching != nil {
				return fmt.Errorf("multiple entries match the source and target topologies")
			}
			matching = transition
		}
	}
	if matching == nil {
		return fmt.Errorf("no transition entry matches source=%+v target=%+v", definition.sourceTopology(), definition.Target)
	}

	available := meta.FindStatusCondition(matching.Evaluations, configv1.TopologyTransitionAvailableConditionType)
	wantAvailable := metav1.ConditionTrue
	if preflight {
		for _, passes := range definition.ExpectedPreflightChecks {
			if !passes {
				wantAvailable = metav1.ConditionFalse
				break
			}
		}
	}
	if available == nil || available.ObservedGeneration != infra.Generation || available.Status != wantAvailable {
		return fmt.Errorf("waiting for current TopologyTransitionAvailable=%s", wantAvailable)
	}
	if wantAvailable == metav1.ConditionFalse && available.Reason != "PreflightCheckFailed" {
		return fmt.Errorf("expected availability reason PreflightCheckFailed, got %s", available.Reason)
	}

	for name, passes := range definition.ExpectedPreflightChecks {
		condition := meta.FindStatusCondition(matching.Evaluations, name)
		want := metav1.ConditionTrue
		if preflight && !passes {
			want = metav1.ConditionFalse
		}
		if condition == nil || condition.ObservedGeneration != infra.Generation || condition.Status != want {
			return fmt.Errorf("waiting for current %s=%s", name, want)
		}
		if want == metav1.ConditionFalse && condition.Reason != "PreflightCheckFailed" {
			return fmt.Errorf("expected %s reason PreflightCheckFailed, got %s", name, condition.Reason)
		}
	}
	for _, condition := range matching.Evaluations {
		if condition.Type == configv1.TopologyTransitionAvailableConditionType {
			continue
		}
		if _, known := definition.ExpectedPreflightChecks[condition.Type]; !known &&
			(condition.Status != metav1.ConditionTrue || condition.ObservedGeneration != infra.Generation) {
			return fmt.Errorf("remaining check %s must be current and True", condition.Type)
		}
	}

	return nil
}

func checkNodeCounts(ctx context.Context, oc *exutil.CLI, target nodeCounts) error {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	controlPlane, compute, arbiter, schedulable := 0, 0, 0, 0
	for _, node := range nodes.Items {
		_, isArbiter := node.Labels["node-role.kubernetes.io/arbiter"]
		_, isWorker := node.Labels["node-role.kubernetes.io/worker"]
		switch {
		case isArbiter:
			arbiter++
		case isControlPlaneNode(node.Labels):
			controlPlane++
			if isWorker && !node.Spec.Unschedulable && nodeIsReady(node) {
				schedulable++
			}
		case isWorker:
			compute++
		default:
			return fmt.Errorf("node %s has no control-plane, worker, or arbiter role", node.Name)
		}
	}
	if controlPlane != target.ControlPlaneNodes || compute != target.ComputeNodes || arbiter != target.ArbiterNodes || schedulable != target.SchedulableControlPlaneNodes {
		return fmt.Errorf("node counts control-plane=%d compute=%d arbiter=%d schedulable-control-plane=%d; want %d/%d/%d/%d",
			controlPlane, compute, arbiter, schedulable, target.ControlPlaneNodes, target.ComputeNodes, target.ArbiterNodes, target.SchedulableControlPlaneNodes)
	}

	return nil
}

func checkEtcdHealthy(ctx context.Context, oc *exutil.CLI) error {
	etcd, err := oc.AdminOperatorClient().OperatorV1().Etcds().Get(ctx, etcdName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if available := v1helpers.FindOperatorCondition(etcd.Status.Conditions, etcdMembersAvailableCondition); available == nil || available.Status != operatorv1.ConditionTrue {
		return fmt.Errorf("etcd %s condition is not True: %s", etcdMembersAvailableCondition, conditionSummary(available))
	}
	if progressing := v1helpers.FindOperatorCondition(etcd.Status.Conditions, etcdMembersProgressingCondition); progressing == nil || progressing.Status != operatorv1.ConditionFalse {
		return fmt.Errorf("etcd %s condition is not False: %s", etcdMembersProgressingCondition, conditionSummary(progressing))
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

func conditionSummary(condition *operatorv1.OperatorCondition) string {
	if condition == nil {
		return "missing"
	}
	return fmt.Sprintf("type=%s status=%s reason=%s", condition.Type, condition.Status, condition.Reason)
}

func isControlPlaneNode(labels map[string]string) bool {
	_, master := labels["node-role.kubernetes.io/master"]
	_, controlPlane := labels["node-role.kubernetes.io/control-plane"]
	return master || controlPlane
}
