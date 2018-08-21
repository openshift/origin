package graphview

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/oc/lib/graph/appsgraph"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
	"github.com/openshift/origin/pkg/oc/lib/graph/kubegraph/analysis"
	kubenodes "github.com/openshift/origin/pkg/oc/lib/graph/kubegraph/nodes"
)

type ReplicationController struct {
	RC *kubenodes.ReplicationControllerNode

	OwnedPods   []*kubenodes.PodNode
	CreatedPods []*kubenodes.PodNode

	ConflictingRCs        []*kubenodes.ReplicationControllerNode
	ConflictingRCIDToPods map[int][]*kubenodes.PodNode
}

// AllReplicationControllers returns all the ReplicationControllers that aren't in the excludes set and the set of covered NodeIDs
func AllReplicationControllers(g osgraph.Graph, excludeNodeIDs IntSet) ([]ReplicationController, IntSet) {
	covered := IntSet{}
	rcViews := []ReplicationController{}

	for _, uncastNode := range g.NodesByKind(kubenodes.ReplicationControllerNodeKind) {
		if excludeNodeIDs.Has(uncastNode.ID()) {
			continue
		}

		rcView, covers := NewReplicationController(g, uncastNode.(*kubenodes.ReplicationControllerNode))
		covered.Insert(covers.List()...)
		rcViews = append(rcViews, rcView)
	}

	return rcViews, covered
}

// MaxRecentContainerRestarts returns the maximum container restarts for all pods in
// replication controller.
func (rc *ReplicationController) MaxRecentContainerRestarts() int32 {
	var maxRestarts int32
	for _, pod := range rc.OwnedPods {
		for _, status := range pod.Status.ContainerStatuses {
			if status.RestartCount > maxRestarts && analysis.ContainerRestartedRecently(status, metav1.Now()) {
				maxRestarts = status.RestartCount
			}
		}
	}
	return maxRestarts
}

// NewReplicationController returns the ReplicationController and a set of all the NodeIDs covered by the ReplicationController
func NewReplicationController(g osgraph.Graph, rcNode *kubenodes.ReplicationControllerNode) (ReplicationController, IntSet) {
	covered := IntSet{}
	covered.Insert(rcNode.ID())

	rcView := ReplicationController{}
	rcView.RC = rcNode
	rcView.ConflictingRCIDToPods = map[int][]*kubenodes.PodNode{}

	for _, uncastPodNode := range g.PredecessorNodesByEdgeKind(rcNode, appsgraph.ManagedByControllerEdgeKind) {
		podNode := uncastPodNode.(*kubenodes.PodNode)
		covered.Insert(podNode.ID())
		rcView.OwnedPods = append(rcView.OwnedPods, podNode)

		// check to see if this pod is managed by more than one RC
		uncastOwningRCs := g.SuccessorNodesByEdgeKind(podNode, appsgraph.ManagedByControllerEdgeKind)
		if len(uncastOwningRCs) > 1 {
			for _, uncastOwningRC := range uncastOwningRCs {
				if uncastOwningRC.ID() == rcNode.ID() {
					continue
				}

				conflictingRC := uncastOwningRC.(*kubenodes.ReplicationControllerNode)
				rcView.ConflictingRCs = append(rcView.ConflictingRCs, conflictingRC)

				conflictingPods, ok := rcView.ConflictingRCIDToPods[conflictingRC.ID()]
				if !ok {
					conflictingPods = []*kubenodes.PodNode{}
				}
				conflictingPods = append(conflictingPods, podNode)
				rcView.ConflictingRCIDToPods[conflictingRC.ID()] = conflictingPods
			}
		}
	}

	return rcView, covered
}

// MaxRecentContainerRestartsForRC returns the maximum container restarts in pods
// in the replication controller node for the last 10 minutes.
func MaxRecentContainerRestartsForRC(g osgraph.Graph, rcNode *kubenodes.ReplicationControllerNode) int32 {
	if rcNode == nil {
		return 0
	}
	rc, _ := NewReplicationController(g, rcNode)
	return rc.MaxRecentContainerRestarts()
}
