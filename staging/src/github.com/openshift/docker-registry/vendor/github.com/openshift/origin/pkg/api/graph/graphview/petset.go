package graphview

import (
	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubeedges "github.com/openshift/origin/pkg/api/kubegraph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
)

type StatefulSet struct {
	StatefulSet *kubegraph.StatefulSetNode

	OwnedPods   []*kubegraph.PodNode
	CreatedPods []*kubegraph.PodNode

	// TODO: handle conflicting once controller refs are present, not worth it yet
}

// AllStatefulSets returns all the StatefulSets that aren't in the excludes set and the set of covered NodeIDs
func AllStatefulSets(g osgraph.Graph, excludeNodeIDs IntSet) ([]StatefulSet, IntSet) {
	covered := IntSet{}
	views := []StatefulSet{}

	for _, uncastNode := range g.NodesByKind(kubegraph.StatefulSetNodeKind) {
		if excludeNodeIDs.Has(uncastNode.ID()) {
			continue
		}

		view, covers := NewStatefulSet(g, uncastNode.(*kubegraph.StatefulSetNode))
		covered.Insert(covers.List()...)
		views = append(views, view)
	}

	return views, covered
}

// NewStatefulSet returns the StatefulSet and a set of all the NodeIDs covered by the StatefulSet
func NewStatefulSet(g osgraph.Graph, node *kubegraph.StatefulSetNode) (StatefulSet, IntSet) {
	covered := IntSet{}
	covered.Insert(node.ID())

	view := StatefulSet{}
	view.StatefulSet = node

	for _, uncastPodNode := range g.PredecessorNodesByEdgeKind(node, kubeedges.ManagedByControllerEdgeKind) {
		podNode := uncastPodNode.(*kubegraph.PodNode)
		covered.Insert(podNode.ID())
		view.OwnedPods = append(view.OwnedPods, podNode)
	}

	return view, covered
}
