package apis

import (
	"context"
	"fmt"
	"time"

	etcdv1alpha1 "github.com/openshift/api/etcd/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	"github.com/openshift/origin/test/extended/two_node/utils/core"
	exutil "github.com/openshift/origin/test/extended/util"
)

// PacemakerCluster GVR for dynamic client access
var pacemakerClusterGVR = schema.GroupVersionResource{
	Group:    "etcd.openshift.io",
	Version:  "v1alpha1",
	Resource: "pacemakerclusters",
}

// pacemakerClusterName is the name of the singleton PacemakerCluster CR
const pacemakerClusterName = "cluster"

// GetPacemakerCluster retrieves the PacemakerCluster CR using dynamic client
func GetPacemakerCluster(oc *exutil.CLI) (*etcdv1alpha1.PacemakerCluster, error) {
	obj, err := oc.AdminDynamicClient().Resource(pacemakerClusterGVR).Get(context.Background(), pacemakerClusterName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get PacemakerCluster: %w", err)
	}

	pc := &etcdv1alpha1.PacemakerCluster{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), pc); err != nil {
		return nil, fmt.Errorf("failed to convert PacemakerCluster: %w", err)
	}
	return pc, nil
}

// PacemakerClusterExists checks if the PacemakerCluster CR exists
func PacemakerClusterExists(oc *exutil.CLI) bool {
	_, err := GetPacemakerCluster(oc)
	return err == nil
}

// findCondition finds a condition by type in a list of conditions
func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// getNodeStatus finds a node's status by name
func getNodeStatus(pc *etcdv1alpha1.PacemakerCluster, nodeName string) *etcdv1alpha1.PacemakerClusterNodeStatus {
	if pc.Status.Nodes == nil {
		return nil
	}
	for i := range *pc.Status.Nodes {
		if (*pc.Status.Nodes)[i].NodeName == nodeName {
			return &(*pc.Status.Nodes)[i]
		}
	}
	return nil
}

// getNodeCount returns the number of nodes in the PacemakerCluster
func getNodeCount(pc *etcdv1alpha1.PacemakerCluster) int {
	if pc.Status.Nodes == nil {
		return 0
	}
	return len(*pc.Status.Nodes)
}

// WaitForPacemakerClusterHealthy waits for the PacemakerCluster to become healthy
func WaitForPacemakerClusterHealthy(oc *exutil.CLI, timeout, pollInterval time.Duration) error {
	klog.V(2).Infof("Waiting for PacemakerCluster to become healthy (timeout: %v)", timeout)

	return core.PollUntil(func() (bool, error) {
		pc, err := GetPacemakerCluster(oc)
		if err != nil {
			klog.V(4).Infof("Error getting PacemakerCluster: %v", err)
			return false, nil
		}

		cond := findCondition(pc.Status.Conditions, etcdv1alpha1.ClusterHealthyConditionType)
		if cond != nil && cond.Status == metav1.ConditionTrue {
			klog.V(2).Infof("PacemakerCluster is healthy")
			return true, nil
		}

		reason := ""
		if cond != nil {
			reason = cond.Reason
		}
		klog.V(4).Infof("PacemakerCluster not yet healthy, reason: %s", reason)
		return false, nil
	}, timeout, pollInterval, "PacemakerCluster to become healthy")
}

// WaitForNodeOffline waits for a specific node to be reported as offline
func WaitForNodeOffline(oc *exutil.CLI, nodeName string, timeout, pollInterval time.Duration) error {
	klog.V(2).Infof("Waiting for node %s to be reported as offline in PacemakerCluster (timeout: %v)", nodeName, timeout)

	return core.PollUntil(func() (bool, error) {
		pc, err := GetPacemakerCluster(oc)
		if err != nil {
			klog.V(4).Infof("Error getting PacemakerCluster: %v", err)
			return false, nil
		}

		nodeStatus := getNodeStatus(pc, nodeName)
		if nodeStatus == nil {
			klog.V(4).Infof("Node %s not found in PacemakerCluster status", nodeName)
			return false, nil
		}

		cond := findCondition(nodeStatus.Conditions, etcdv1alpha1.NodeOnlineConditionType)
		if cond == nil {
			klog.V(4).Infof("Online condition not found for node %s", nodeName)
			return false, nil
		}

		if cond.Status == metav1.ConditionFalse && cond.Reason == etcdv1alpha1.NodeOnlineReasonOffline {
			klog.V(2).Infof("Node %s is now reported as offline", nodeName)
			return true, nil
		}

		klog.V(4).Infof("Node %s Online condition: %s (reason: %s)", nodeName, cond.Status, cond.Reason)
		return false, nil
	}, timeout, pollInterval, fmt.Sprintf("node %s to be offline", nodeName))
}

// WaitForNodeCount waits for a specific number of nodes in the PacemakerCluster
func WaitForNodeCount(oc *exutil.CLI, expectedCount int, timeout, pollInterval time.Duration) error {
	klog.V(2).Infof("Waiting for PacemakerCluster to have %d nodes (timeout: %v)", expectedCount, timeout)

	return core.PollUntil(func() (bool, error) {
		pc, err := GetPacemakerCluster(oc)
		if err != nil {
			klog.V(4).Infof("Error getting PacemakerCluster: %v", err)
			return false, nil
		}

		actualCount := getNodeCount(pc)
		if actualCount == expectedCount {
			klog.V(2).Infof("PacemakerCluster has %d nodes", expectedCount)
			return true, nil
		}

		klog.V(4).Infof("PacemakerCluster has %d nodes, waiting for %d", actualCount, expectedCount)
		return false, nil
	}, timeout, pollInterval, fmt.Sprintf("PacemakerCluster to have %d nodes", expectedCount))
}

// LogPacemakerClusterStatus logs the current PacemakerCluster status for debugging
func LogPacemakerClusterStatus(oc *exutil.CLI, context string) {
	pc, err := GetPacemakerCluster(oc)
	if err != nil {
		klog.V(2).Infof("[%s] Failed to get PacemakerCluster status: %v", context, err)
		return
	}

	healthyCond := findCondition(pc.Status.Conditions, etcdv1alpha1.ClusterHealthyConditionType)
	nodeCountCond := findCondition(pc.Status.Conditions, etcdv1alpha1.ClusterNodeCountAsExpectedConditionType)

	klog.V(2).Infof("[%s] PacemakerCluster status:", context)
	klog.V(2).Infof("  LastUpdated: %s", pc.Status.LastUpdated.Format(time.RFC3339))
	if healthyCond != nil {
		klog.V(2).Infof("  Healthy: %s (reason: %s)", healthyCond.Status, healthyCond.Reason)
	}
	if nodeCountCond != nil {
		klog.V(2).Infof("  NodeCountAsExpected: %s (reason: %s)", nodeCountCond.Status, nodeCountCond.Reason)
	}
	klog.V(2).Infof("  Node count: %d", getNodeCount(pc))

	if pc.Status.Nodes != nil {
		for _, node := range *pc.Status.Nodes {
			healthyCond := findCondition(node.Conditions, etcdv1alpha1.NodeHealthyConditionType)
			onlineCond := findCondition(node.Conditions, etcdv1alpha1.NodeOnlineConditionType)

			healthyStr := "unknown"
			if healthyCond != nil {
				healthyStr = fmt.Sprintf("%s (%s)", healthyCond.Status, healthyCond.Reason)
			}
			onlineStr := "unknown"
			if onlineCond != nil {
				onlineStr = fmt.Sprintf("%s (%s)", onlineCond.Status, onlineCond.Reason)
			}

			klog.V(2).Infof("  Node %s: Healthy=%s, Online=%s", node.NodeName, healthyStr, onlineStr)
		}
	}
}
