package graphview

import (
	appsedges "github.com/openshift/origin/pkg/oc/graph/appsgraph"
	osgraph "github.com/openshift/origin/pkg/oc/graph/genericgraph"
	kubeedges "github.com/openshift/origin/pkg/oc/graph/kubegraph"
	kubegraph "github.com/openshift/origin/pkg/oc/graph/kubegraph/nodes"
)

type StatefulSet struct {
	StatefulSet *kubegraph.StatefulSetNode

	OwnedPods   []*kubegraph.PodNode
	CreatedPods []*kubegraph.PodNode

	Images []ImagePipeline

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

	for _, istNode := range g.PredecessorNodesByEdgeKind(node, kubeedges.TriggersDeploymentEdgeKind) {
		imagePipeline, covers := NewImagePipelineFromImageTagLocation(g, istNode, istNode.(ImageTagLocation))
		covered.Insert(covers.List()...)
		view.Images = append(view.Images, imagePipeline)
	}

	// for image that we use, create an image pipeline and add it to the list
	for _, tagNode := range g.PredecessorNodesByEdgeKind(node, appsedges.UsedInDeploymentEdgeKind) {
		imagePipeline, covers := NewImagePipelineFromImageTagLocation(g, tagNode, tagNode.(ImageTagLocation))

		covered.Insert(covers.List()...)
		view.Images = append(view.Images, imagePipeline)
	}

	return view, covered
}
