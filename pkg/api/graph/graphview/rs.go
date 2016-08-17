package graphview

import (
	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubeedges "github.com/openshift/origin/pkg/api/kubegraph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
)

type ReplicaSet struct {
	RS *kubegraph.ReplicaSetNode

	OwnedPods   []*kubegraph.PodNode
	CreatedPods []*kubegraph.PodNode

	ConflictingRSs        []*kubegraph.ReplicaSetNode
	ConflictingRSIDToPods map[int][]*kubegraph.PodNode
}

// AllReplicaSets returns all the ReplicaSets that aren't in the excludes set and the set of covered NodeIDs
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

// MaxRecentContainerRestarts returns the maximum container restarts for all pods in
// replication controller.
func (rs *ReplicaSet) MaxRecentContainerRestarts() int32 {
	return MaxRecentContainerRestartsForPods(rs.OwnedPods)
}

// NewReplicaSet returns the ReplicaSet and a set of all the NodeIDs covered by the ReplicaSet
func NewReplicaSet(g osgraph.Graph, rsNode *kubegraph.ReplicaSetNode) (ReplicaSet, IntSet) {
	covered := IntSet{}
	covered.Insert(rsNode.ID())

	rsView := ReplicaSet{}
	rsView.RS = rsNode
	rsView.ConflictingRSIDToPods = map[int][]*kubegraph.PodNode{}

	for _, uncastPodNode := range g.PredecessorNodesByEdgeKind(rsNode, kubeedges.ManagedByControllerEdgeKind) {
		podNode := uncastPodNode.(*kubegraph.PodNode)
		covered.Insert(podNode.ID())
		rsView.OwnedPods = append(rsView.OwnedPods, podNode)

		// check to see if this pod is managed by more than one RS
		uncastOwningRSs := g.SuccessorNodesByEdgeKind(podNode, kubeedges.ManagedByControllerEdgeKind)
		if len(uncastOwningRSs) > 1 {
			for _, uncastOwningRS := range uncastOwningRSs {
				if uncastOwningRS.ID() == rsNode.ID() {
					continue
				}

				conflictingRS := uncastOwningRS.(*kubegraph.ReplicaSetNode)
				rsView.ConflictingRSs = append(rsView.ConflictingRSs, conflictingRS)

				conflictingPods, ok := rsView.ConflictingRSIDToPods[conflictingRS.ID()]
				if !ok {
					conflictingPods = []*kubegraph.PodNode{}
				}
				conflictingPods = append(conflictingPods, podNode)
				rsView.ConflictingRSIDToPods[conflictingRS.ID()] = conflictingPods
			}
		}
	}

	return rsView, covered
}

// MaxRecentContainerRestartsForRS returns the maximum container restarts in pods
// in the replica set node for the last 10 minutes.
func MaxRecentContainerRestartsForRS(g osgraph.Graph, rsNode *kubegraph.ReplicaSetNode) int32 {
	if rsNode == nil {
		return 0
	}
	rs, _ := NewReplicaSet(g, rsNode)
	return rs.MaxRecentContainerRestarts()
}
