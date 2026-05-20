package apis

import (
	"context"
	"fmt"
	"time"

	etcdv1alpha1 "github.com/openshift/api/etcd/v1alpha1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var PacemakerClusterGVR = schema.GroupVersionResource{
	Group: etcdv1alpha1.GroupName, Version: "v1alpha1", Resource: "pacemakerclusters",
}

func IsPacemakerClusterAvailable(oc *exutil.CLI) bool {
	_, err := oc.AdminDynamicClient().Resource(PacemakerClusterGVR).List(
		context.Background(), metav1.ListOptions{Limit: 1})
	return err == nil
}

func GetPacemakerCluster(oc *exutil.CLI) (*etcdv1alpha1.PacemakerCluster, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	u, err := oc.AdminDynamicClient().Resource(PacemakerClusterGVR).Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get PacemakerCluster: %w", err)
	}
	var pc etcdv1alpha1.PacemakerCluster
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &pc); err != nil {
		return nil, fmt.Errorf("convert PacemakerCluster: %w", err)
	}
	return &pc, nil
}

func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

func ExpectClusterHealthy(pc *etcdv1alpha1.PacemakerCluster) error {
	c := findCondition(pc.Status.Conditions, etcdv1alpha1.ClusterHealthyConditionType)
	if c == nil {
		return fmt.Errorf("PacemakerCluster missing %s condition", etcdv1alpha1.ClusterHealthyConditionType)
	}
	if c.Status != metav1.ConditionTrue {
		return fmt.Errorf("PacemakerCluster %s=%s (reason: %s, message: %s)",
			etcdv1alpha1.ClusterHealthyConditionType, c.Status, c.Reason, c.Message)
	}
	return nil
}

func ExpectNodeFencingHealthy(pc *etcdv1alpha1.PacemakerCluster, nodeName string) error {
	if pc.Status.Nodes == nil {
		return fmt.Errorf("PacemakerCluster has no nodes in status")
	}
	for _, node := range *pc.Status.Nodes {
		if node.NodeName != nodeName {
			continue
		}
		c := findCondition(node.Conditions, etcdv1alpha1.NodeFencingHealthyConditionType)
		if c == nil {
			return fmt.Errorf("node %s missing %s condition", nodeName, etcdv1alpha1.NodeFencingHealthyConditionType)
		}
		if c.Status != metav1.ConditionTrue {
			return fmt.Errorf("node %s %s=%s (reason: %s, message: %s)",
				nodeName, etcdv1alpha1.NodeFencingHealthyConditionType, c.Status, c.Reason, c.Message)
		}
		return nil
	}
	return fmt.Errorf("node %s not found in PacemakerCluster status", nodeName)
}
