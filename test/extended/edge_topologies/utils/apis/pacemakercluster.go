package apis

import (
	"context"
	"fmt"
	"time"

	etcdv1 "github.com/openshift/api/etcd/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var PacemakerClusterGVR = schema.GroupVersionResource{
	Group: etcdv1.GroupName, Version: "v1", Resource: "pacemakerclusters",
}

// IsPacemakerClusterAvailable reports whether the PacemakerCluster CRD is served
// by the API. Only a NotFound error means the CRD is genuinely absent; any other
// error (authorization, transient API failures) is returned so callers fail
// rather than silently skip checks by mistaking a real error for CRD absence.
func IsPacemakerClusterAvailable(oc *exutil.CLI) (bool, error) {
	_, err := oc.AdminDynamicClient().Resource(PacemakerClusterGVR).List(
		context.Background(), metav1.ListOptions{Limit: 1})
	if err == nil {
		return true, nil
	}
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("check PacemakerCluster availability: %w", err)
}

func GetPacemakerCluster(oc *exutil.CLI) (*etcdv1.PacemakerCluster, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	u, err := oc.AdminDynamicClient().Resource(PacemakerClusterGVR).Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get PacemakerCluster: %w", err)
	}
	var pc etcdv1.PacemakerCluster
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &pc); err != nil {
		return nil, fmt.Errorf("convert PacemakerCluster: %w", err)
	}
	return &pc, nil
}

// ExpectClusterCondition returns nil only when the PacemakerCluster's cluster-level
// condition condType is explicitly set to expected. A missing condition is
// propagated as an error so a lookup failure can never be mistaken for the
// desired state.
func ExpectClusterCondition(pc *etcdv1.PacemakerCluster, condType string, expected metav1.ConditionStatus) error {
	c := meta.FindStatusCondition(pc.Status.Conditions, condType)
	if c == nil {
		return fmt.Errorf("PacemakerCluster missing %s condition", condType)
	}
	if c.Status != expected {
		return fmt.Errorf("PacemakerCluster %s=%s, expected %s (reason: %s, message: %s)",
			condType, c.Status, expected, c.Reason, c.Message)
	}
	return nil
}

// ExpectNodeCondition returns nil only when nodeName's condition condType is
// explicitly set to expected. Missing nodes, missing conditions, and other
// schema errors are propagated so a lookup failure can never be mistaken for
// the desired state.
func ExpectNodeCondition(pc *etcdv1.PacemakerCluster, nodeName, condType string, expected metav1.ConditionStatus) error {
	if pc.Status.Nodes == nil {
		return fmt.Errorf("PacemakerCluster has no nodes in status")
	}
	for _, node := range *pc.Status.Nodes {
		if node.NodeName != nodeName {
			continue
		}
		c := meta.FindStatusCondition(node.Conditions, condType)
		if c == nil {
			return fmt.Errorf("node %s missing %s condition", nodeName, condType)
		}
		if c.Status != expected {
			return fmt.Errorf("node %s %s=%s, expected %s (reason: %s, message: %s)",
				nodeName, condType, c.Status, expected, c.Reason, c.Message)
		}
		return nil
	}
	return fmt.Errorf("node %s not found in PacemakerCluster status", nodeName)
}

func ExpectClusterHealthy(pc *etcdv1.PacemakerCluster) error {
	return ExpectClusterCondition(pc, etcdv1.ClusterHealthyConditionType, metav1.ConditionTrue)
}

func ExpectNodeFencingAvailable(pc *etcdv1.PacemakerCluster, nodeName string) error {
	return ExpectNodeCondition(pc, nodeName, etcdv1.NodeFencingAvailableConditionType, metav1.ConditionTrue)
}

// ExpectNodeFencingUnavailable returns nil only when the node's
// NodeFencingAvailable condition is explicitly False.
func ExpectNodeFencingUnavailable(pc *etcdv1.PacemakerCluster, nodeName string) error {
	return ExpectNodeCondition(pc, nodeName, etcdv1.NodeFencingAvailableConditionType, metav1.ConditionFalse)
}

// ExpectNodeFencingUnhealthy returns nil only when the node's
// NodeFencingHealthy condition is explicitly False (e.g. a fencing agent is
// unmanaged but still running).
func ExpectNodeFencingUnhealthy(pc *etcdv1.PacemakerCluster, nodeName string) error {
	return ExpectNodeCondition(pc, nodeName, etcdv1.NodeFencingHealthyConditionType, metav1.ConditionFalse)
}

// ExpectNodeOnlineFalse returns nil only when the node's Online condition is
// explicitly False.
func ExpectNodeOnlineFalse(pc *etcdv1.PacemakerCluster, nodeName string) error {
	return ExpectNodeCondition(pc, nodeName, etcdv1.NodeOnlineConditionType, metav1.ConditionFalse)
}

func ExpectNodeMember(pc *etcdv1.PacemakerCluster, nodeName string) error {
	return ExpectNodeCondition(pc, nodeName, etcdv1.NodeMemberConditionType, metav1.ConditionTrue)
}

func ExpectClusterNodeCountAsExpected(pc *etcdv1.PacemakerCluster) error {
	return ExpectClusterCondition(pc, etcdv1.ClusterNodeCountAsExpectedConditionType, metav1.ConditionTrue)
}

// ExpectPacemakerCRFresh returns nil only when the PacemakerCluster CR's
// Status.LastUpdated is set and within maxAge of now. A zero/missing
// LastUpdated or one older than maxAge means the status-collector pipeline
// has stalled, so the CR can no longer be trusted to reflect live state.
func ExpectPacemakerCRFresh(pc *etcdv1.PacemakerCluster, maxAge time.Duration) error {
	lastUpdated := pc.Status.LastUpdated.Time
	if lastUpdated.IsZero() {
		return fmt.Errorf("PacemakerCluster CR lastUpdated is zero (status never populated)")
	}
	if age := time.Since(lastUpdated); age > maxAge {
		return fmt.Errorf("PacemakerCluster CR is stale: lastUpdated %s ago, expected < %s", age.Round(time.Second), maxAge)
	}
	return nil
}

// ExpectPacemakerBaseline asserts the PacemakerCluster CR reflects a fully
// healthy cluster: cluster-level Healthy, InService, and NodeCountAsExpected
// conditions, plus per-node FencingAvailable, FencingHealthy, and Member
// conditions for every node currently reported in status. It is the test
// payload for the dedicated PacemakerHealthCheck/fencing-credentials tests,
// not a skip gate — it is built on etcdv1 condition type constants, so it
// only breaks on an actual API break to the GA'd v1 type, not when
// cluster-etcd-operator adds new conditions.
func ExpectPacemakerBaseline(oc *exutil.CLI) error {
	pc, err := GetPacemakerCluster(oc)
	if err != nil {
		return err
	}

	if err := ExpectClusterHealthy(pc); err != nil {
		return err
	}
	if err := ExpectClusterCondition(pc, etcdv1.ClusterInServiceConditionType, metav1.ConditionTrue); err != nil {
		return err
	}
	if err := ExpectClusterNodeCountAsExpected(pc); err != nil {
		return err
	}

	if pc.Status.Nodes == nil {
		return fmt.Errorf("PacemakerCluster has no nodes in status")
	}
	for _, node := range *pc.Status.Nodes {
		if err := ExpectNodeFencingAvailable(pc, node.NodeName); err != nil {
			return err
		}
		if err := ExpectNodeFencingHealthy(pc, node.NodeName); err != nil {
			return err
		}
		if err := ExpectNodeMember(pc, node.NodeName); err != nil {
			return err
		}
	}
	return nil
}

func ExpectNodeFencingHealthy(pc *etcdv1.PacemakerCluster, nodeName string) error {
	return ExpectNodeCondition(pc, nodeName, etcdv1.NodeFencingHealthyConditionType, metav1.ConditionTrue)
}

// FindStartedFencingAgent returns the name of a fencing agent that targets nodeName
// (e.g. "master-0_redfish") and is currently reporting Started=True. This reads the
// PacemakerCluster CR's Status.Nodes[].FencingAgents — the structured source of truth
// for fencing agent identity — so callers don't need to parse pcs status text.
func FindStartedFencingAgent(pc *etcdv1.PacemakerCluster, nodeName string) (string, error) {
	if pc.Status.Nodes == nil {
		return "", fmt.Errorf("PacemakerCluster has no nodes in status")
	}
	for _, node := range *pc.Status.Nodes {
		if node.NodeName != nodeName {
			continue
		}
		for _, agent := range node.FencingAgents {
			if c := meta.FindStatusCondition(agent.Conditions, etcdv1.ResourceStartedConditionType); c != nil && c.Status == metav1.ConditionTrue {
				return agent.Name, nil
			}
		}
		return "", fmt.Errorf("no started fencing agent found for node %s", nodeName)
	}
	return "", fmt.Errorf("node %s not found in PacemakerCluster status", nodeName)
}
