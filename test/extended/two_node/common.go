package two_node

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	"github.com/openshift/origin/test/extended/util"
	exutil "github.com/openshift/origin/test/extended/util"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

const (
	labelNodeRoleMaster       = "node-role.kubernetes.io/master"
	labelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
	labelNodeRoleWorker       = "node-role.kubernetes.io/worker"
	labelNodeRoleArbiter      = "node-role.kubernetes.io/arbiter"

	clusterOperatorIsHealthyTimeout = time.Minute
	nodeIsHealthyTimeout            = time.Minute
	resourceIsHealthyTimeout        = time.Minute
)

func skipIfNotTopology(oc *exutil.CLI, wanted v1.TopologyMode) {
	current, err := exutil.GetControlPlaneTopology(oc)
	if err != nil {
		e2eskipper.Skip(fmt.Sprintf("Could not get current topology, skipping test: error %v", err))
	}
	if *current != wanted {
		e2eskipper.Skip(fmt.Sprintf("Cluster is not in %v topology, skipping test", wanted))
	}
}

func skipIfClusterIsNotHealthy(oc *util.CLI, ecf *helpers.EtcdClientFactoryImpl, nodes *corev1.NodeList) {
	framework.Logf("Ensure Etcd pods are running")
	err := ensureEtcdPodsAreRunning(oc)
	if err != nil {
		e2eskipper.Skip(fmt.Sprintf("could not ensure etcd pods are running: %v", err))
	}

	framework.Logf("Ensure Etcd member list has two voting members")
	err = ensureEtcdHasTwoVotingMembers(nodes, ecf)
	if err != nil {
		e2eskipper.Skip(fmt.Sprintf("could not ensure etcd has two voting members: %v", err))
	}

	framework.Logf("Ensure cluster operator is healthy")
	err = ensureClusterOperatorHealthy(oc)
	if err != nil {
		e2eskipper.Skip(fmt.Sprintf("could not ensure cluster-operator is healthy: %v", err))
	}
}

func isClusterOperatorAvailable(operator *v1.ClusterOperator) bool {
	for _, cond := range operator.Status.Conditions {
		if cond.Type == v1.OperatorAvailable && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func isClusterOperatorDegraded(operator *v1.ClusterOperator) bool {
	for _, cond := range operator.Status.Conditions {
		if cond.Type == v1.OperatorDegraded && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func hasNodeRebooted(oc *util.CLI, node *corev1.Node) (bool, error) {
	if n, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{}); err != nil {
		return false, err
	} else {
		return n.Status.NodeInfo.BootID != node.Status.NodeInfo.BootID, nil
	}
}

// ensureClusterOperatorHealthy checks if the cluster-etcd-operator is healthy before running etcd tests
func ensureClusterOperatorHealthy(oc *util.CLI) error {
	ctx, cancel := context.WithTimeout(context.Background(), clusterOperatorIsHealthyTimeout)
	defer cancel()

	var err error
	var co *v1.ClusterOperator
	for {
		if co, err = oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, "etcd", metav1.GetOptions{}); err != nil {
			err = fmt.Errorf("failed to retrieve ClusterOperator: %v", err)
		} else {
			// Check if etcd operator is Available
			available := findClusterOperatorCondition(co.Status.Conditions, v1.OperatorAvailable)
			if available == nil {
				err = fmt.Errorf("ClusterOperator Available condition not found")
			} else if available.Status != v1.ConditionTrue {
				err = fmt.Errorf("ClusterOperator is not Available: %s", available.Message)
			} else {
				// Check if etcd operator is not Degraded
				degraded := findClusterOperatorCondition(co.Status.Conditions, v1.OperatorDegraded)
				if degraded != nil && degraded.Status == v1.ConditionTrue {
					err = fmt.Errorf("ClusterOperator is Degraded: %s", degraded.Message)
				} else {
					return nil
				}
			}
		}

		select {
		case <-ctx.Done():
			return err
		default:
		}
		time.Sleep(pollInterval)
	}
}

func ensureEtcdPodsAreRunning(oc *util.CLI) error {
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

func ensureEtcdHasTwoVotingMembers(nodes *corev1.NodeList, ecf *helpers.EtcdClientFactoryImpl) error {
	ctx, cancel := context.WithTimeout(context.Background(), resourceIsHealthyTimeout)
	defer cancel()

	for {
		var err error
		var members []*etcdserverpb.Member

		// Check all conditions sequentially
		members, err = getMembers(ecf)
		if err == nil && len(members) != 2 {
			err = fmt.Errorf("expected 2 members, found %d", len(members))
		}

		if err == nil {
			for _, node := range nodes.Items {
				isStarted, isLearner, checkErr := getMemberState(&node, members)
				if checkErr != nil {
					err = checkErr
				} else if !isStarted || isLearner {
					err = fmt.Errorf("member %s is not a voting member (started=%v, learner=%v)",
						node.Name, isStarted, isLearner)
					break
				}
			}

		}

		// All checks passed - success!
		if err == nil {
			framework.Logf("SUCCESS: got membership with two voting members: %+v", members)
			return nil
		}

		// Checks failed - evaluate timeout
		select {
		case <-ctx.Done():
			return fmt.Errorf("etcd membership does not have two voters: %v, membership: %+v", err, members)
		default:
		}
		time.Sleep(pollInterval)
	}
}
