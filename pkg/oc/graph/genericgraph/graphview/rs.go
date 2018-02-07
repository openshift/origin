package graphview

import (
	osgraph "github.com/openshift/origin/pkg/oc/graph/genericgraph"
	kubeedges "github.com/openshift/origin/pkg/oc/graph/kubegraph"
	"github.com/openshift/origin/pkg/oc/graph/kubegraph/analysis"
	kubegraph "github.com/openshift/origin/pkg/oc/graph/kubegraph/nodes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ReplicaSet struct {
	RS *kubegraph.ReplicaSetNode

	OwnedPods   []*kubegraph.PodNode
	CreatedPods []*kubegraph.PodNode
}

func AllReplicaSets(g osgraph.Graph, excludeNodeIDs IntSet) ([]ReplicaSet, IntSet) {
	covered := IntSet{}
	rsViews := []ReplicaSet{}

	for _, uncastNode := range g.NodesByKind(kubegraph.ReplicaSetNodeKind) {
		if excludeNodeIDs.Has(uncastNode.ID()) {
			continue
		}

		rsView, covers := NewReplicaSet(g, uncastNode.(*kubegraph.ReplicaSetNode))
		covered.Insert(covers.List()...)
		rsViews = append(rsViews, rsView)
	}

	return rsViews, covered
}

// MaxRecentContainerRestarts returns the maximum container restarts for all pods
func (rs *ReplicaSet) MaxRecentContainerRestarts() int32 {
	var maxRestarts int32
	for _, pod := range rs.OwnedPods {
		for _, status := range pod.Status.ContainerStatuses {
			if status.RestartCount > maxRestarts && analysis.ContainerRestartedRecently(status, metav1.Now()) {
				maxRestarts = status.RestartCount
			}
		}
	}
	return maxRestarts
}

// NewReplicationController returns the ReplicationController and a set of all the NodeIDs covered by the ReplicationController
func NewReplicaSet(g osgraph.Graph, rsNode *kubegraph.ReplicaSetNode) (ReplicaSet, IntSet) {
	covered := IntSet{}
	covered.Insert(rsNode.ID())

	rsView := ReplicaSet{}
	rsView.RS = rsNode

	for _, uncastPodNode := range g.PredecessorNodesByEdgeKind(rsNode, kubeedges.ManagedByControllerEdgeKind) {
		podNode := uncastPodNode.(*kubegraph.PodNode)
		covered.Insert(podNode.ID())
		rsView.OwnedPods = append(rsView.OwnedPods, podNode)
	}

	return rsView, covered
}

func MaxRecentContainerRestartsForRS(g osgraph.Graph, rsNode *kubegraph.ReplicaSetNode) int32 {
	if rsNode == nil {
		return 0
	}
	rs, _ := NewReplicaSet(g, rsNode)
	return rs.MaxRecentContainerRestarts()
}
